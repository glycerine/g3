//go:build g3trace

package pdr

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestInductiveModelTracePrecedesCertificateConversion(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	baseManager, spareOwner, transformer, _, closeBase := closureFixture(t)
	globals := spareOwner.contexts.config.Globals
	var output bytes.Buffer
	tags := diagnostic.NewTraceTagsWithWriter(&output)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: baseManager.Symbols, Numerals: baseManager.NumeralService(), Budget: baseManager.Budget(), TraceTags: tags, Params: globals.Context, Globals: globals})
	must(err)
	must(manager.RegisterDeclarationPlugins())
	context, err := NewContext(manager, nil, 2, ReachableConstraintCache, smtcore.SMTSolverConfig{Params: globals.Context, Globals: globals})
	must(err)
	boolean, err := manager.MkBoolSort()
	must(err)
	decl := func(name string) ast.ASTHandle {
		s, x := manager.Symbols.Intern(name)
		must(x)
		d, x := manager.MkFuncDecl(s, nil, boolean, ast.ASTFuncDeclInfo{})
		must(x)
		return d
	}
	normalized := decl("trace_raw_predicate")
	original := decl("trace_converted_predicate")
	query := decl("trace_query_predicate")
	converter := &certificatePredicateConverter{original: original, normalized: normalized}
	must(context.SetCertificateModelConverter(converter))
	rm, err := base.NewRuleManager(manager)
	must(err)
	must(rm.RegisterPredicate(normalized))
	must(rm.RegisterPredicate(query))
	rules, err := base.NewRuleSet(rm)
	must(err)
	add := func(d ast.ASTHandle, tail []ast.ASTHandle) {
		h, x := manager.MkApp(d, nil)
		must(x)
		r, x := rm.NewRule(h, tail, nil, "trace-rule")
		must(x)
		must(rules.Add(r))
		must(r.Close())
	}
	add(normalized, nil)
	falsity, err := manager.MkFalse()
	must(err)
	add(query, []ast.ASTHandle{falsity})
	must(context.Initialize(rules))
	status, err := context.Solve(query)
	must(err)
	if status != lbool.False {
		t.Fatal("expected inductive result", status)
	}
	before, err := context.Model(query)
	must(err)
	must(before.Close())
	if output.Len() != 0 {
		t.Fatalf("disabled trace emitted output: %q", output.String())
	}
	tags.Enable("pdr")
	var certificate bytes.Buffer
	must(context.DisplayCertificate(&certificate, query, false))
	trace := output.String()
	if !strings.Contains(trace, "src/muz/pdr/pdr_manager.cpp:109") || !strings.Contains(trace, "trace_raw_predicate") || strings.Contains(trace, "trace_converted_predicate") || converter.calls != 1 || !strings.Contains(certificate.String(), "trace_converted_predicate") {
		t.Fatalf("raw trace / converted certificate mismatch: trace=%q certificate=%q calls=%d", trace, certificate.String(), converter.calls)
	}
	output.Reset()
	must(context.validateUNSATResult())
	if !strings.Contains(output.String(), "trace_raw_predicate") {
		t.Fatalf("validation omitted raw model trace: %q", output.String())
	}
	output.Reset()
	answer, err := context.Answer(query)
	must(err)
	if !strings.Contains(output.String(), "trace_raw_predicate") {
		t.Fatalf("answer omitted raw model trace: %q", output.String())
	}
	must(manager.DelRef(&answer))
	tags.Disable("pdr")
	must(context.Close())
	must(rules.Close())
	must(rm.Close())
	must(manager.Close())
	must(transformer.Close())
	must(spareOwner.Close())
	closeBase()
}
