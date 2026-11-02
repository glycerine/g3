package api

import (
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestAPIFixedpointPendingDisplayPreservesBindersAcrossFlushAndScopes(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	b := f.pin(f.m.MkBoolSort())
	p, err := f.fp.DeclareRelation("pending-p", []ast.ASTHandle{b})
	astTestMust(t, err)
	v := f.pin(f.m.MkVar(0, b))
	app := f.pin(f.m.MkApp(p, []ast.ASTHandle{v}))
	makeRule := func(name string) ast.ASTHandle {
		return f.pin(f.m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []usymbol.Symbol{astTestSymbol(t, f.m, name)}, Body: app, Weight: 7}))
	}
	options, err := f.c.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, options.SetBool("print_with_variable_declarations", false))
	astTestMust(t, f.fp.SetParameters(options))
	astTestMust(t, options.Close())
	astTestMust(t, f.fp.AddRule(makeRule("source-first"), "first"))
	check := func(want, absent string) string {
		before := f.m.Stats().LiveReferences
		text, err := f.fp.SMT2()
		astTestMust(t, err)
		if want != "" && !strings.Contains(text, want) || absent != "" && strings.Contains(text, absent) {
			t.Fatalf("want %q absent %q in %s", want, absent, text)
		}
		if f.m.Stats().LiveReferences != before {
			t.Fatal("display leaked references")
		}
		return text
	}
	first := check("source-first", "")
	if !strings.Contains(first, ":weight 7") {
		t.Fatal("pending weight lost", first)
	}
	astTestMust(t, f.fp.Push())
	astTestMust(t, f.fp.AddRule(makeRule("source-scoped"), "scoped"))
	check("source-scoped", "")
	astTestMust(t, f.fp.Pop())
	check("source-first", "source-scoped")
	status, err := f.fp.Query(p)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	check("pending-p", "source-first")
	astTestMust(t, f.fp.AddRule(makeRule("source-next"), "next"))
	mixed := check("source-next", "source-first")
	if strings.Count(mixed, "(rule ") != 2 {
		t.Fatal("pending normalized duplicate", mixed)
	}
	status, err = f.fp.Query(p)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	check("pending-p", "source-next")
}

func TestAPIFixedpointPendingDisplayExpansionAndUpdateRanges(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	astTestMust(t, f.fp.AddRule(f.pAtom, "replace"))
	disjunction := f.pin(f.m.MkOrArgs([]ast.ASTHandle{f.pAtom, f.qAtom}))
	rule := f.pin(f.m.MkImplies(disjunction, f.uAtom))
	astTestMust(t, f.fp.AddRule(rule, "expanded"))
	astTestMust(t, f.fp.UpdateRule(f.pAtom, "replace"))
	text, err := f.fp.SMT2()
	astTestMust(t, err)
	if strings.Count(text, "(rule ") != 2 || !strings.Contains(text, "(or api-object-fp-p api-object-fp-q)") {
		t.Fatal("expanded input duplicated or update shifted its origin", text)
	}
	status, err := f.fp.Query(f.u)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	text, err = f.fp.SMT2()
	astTestMust(t, err)
	if strings.Count(text, "(rule ") != 3 {
		t.Fatal("normalized split clauses missing", text)
	}
}

func TestAPIFixedpointDualityQueryKeepsClosedRawDisplay(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	b := f.pin(f.m.MkBoolSort())
	p, err := f.fp.DeclareRelation("duality-pending", []ast.ASTHandle{b})
	astTestMust(t, err)
	v := f.pin(f.m.MkVar(0, b))
	app := f.pin(f.m.MkApp(p, []ast.ASTHandle{v}))
	rule := f.pin(f.m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []usymbol.Symbol{astTestSymbol(t, f.m, "duality-original")}, Body: app}))
	astTestMust(t, f.fp.AddRule(rule, "original"))
	options, err := f.c.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, options.SetSymbol("engine", "duality"))
	astTestMust(t, options.SetBool("print_with_variable_declarations", false))
	astTestMust(t, f.fp.SetParameters(options))
	astTestMust(t, options.Close())
	status, err := f.fp.Query(p)
	astTestMust(t, err)
	if status != lbool.True {
		t.Fatal(status)
	}
	text, err := f.fp.SMT2()
	astTestMust(t, err)
	if !strings.Contains(text, "duality-original") {
		t.Fatal("ordinary Duality query flushed raw input", text)
	}
}
