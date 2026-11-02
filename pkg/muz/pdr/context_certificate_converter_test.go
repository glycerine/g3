package pdr

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	utext "github.com/glycerine/g3/pkg/util/text"
)

type certificatePredicateConverter struct {
	original, normalized ast.ASTHandle
	calls                int
}

func (c *certificatePredicateConverter) ConvertCertificateModel(source *model.Model) (*model.Model, error) {
	c.calls++
	value, err := source.Core().ConstantInterpretation(c.normalized)
	if err != nil {
		return nil, err
	}
	if err = source.Core().RegisterConstant(c.original, value); err != nil {
		return nil, err
	}
	if err = source.Core().UnregisterDeclaration(c.normalized); err != nil {
		return nil, err
	}
	return source, nil
}

func TestContextCertificateConvertsTraceButLeavesVerboseFiniteModelRaw(t *testing.T) {
	manager, spareOwner, transformer, _, closeBase := closureFixture(t)
	context, err := NewContext(manager, nil, 2, ReachableConstraintCache, spareOwner.contexts.config)
	closureMust(t, err)
	boolean, err := manager.MkBoolSort()
	closureMust(t, err)
	declaration := func(name string) ast.ASTHandle {
		symbol, x := manager.Symbols.Intern(name)
		closureMust(t, x)
		result, x := manager.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
		closureMust(t, x)
		return result
	}
	original := declaration("certificate_original_predicate")
	normalized := declaration("certificate_preprocessed_predicate")
	query := declaration("certificate_query_predicate")
	converter := &certificatePredicateConverter{original: original, normalized: normalized}
	closureMust(t, context.SetCertificateModelConverter(converter))
	ruleManager, err := base.NewRuleManager(manager)
	closureMust(t, err)
	closureMust(t, ruleManager.RegisterPredicate(normalized))
	closureMust(t, ruleManager.RegisterPredicate(query))
	rules, err := base.NewRuleSet(ruleManager)
	closureMust(t, err)
	normalizedHead, err := manager.MkApp(normalized, nil)
	closureMust(t, err)
	fact, err := ruleManager.NewRule(normalizedHead, nil, nil, "normalized-fact")
	closureMust(t, err)
	closureMust(t, rules.Add(fact))
	closureMust(t, fact.Close())
	queryHead, err := manager.MkApp(query, nil)
	closureMust(t, err)
	falsity, err := manager.MkFalse()
	closureMust(t, err)
	blocked, err := ruleManager.NewRule(queryHead, []ast.ASTHandle{falsity}, nil, "blocked-query")
	closureMust(t, err)
	closureMust(t, rules.Add(blocked))
	closureMust(t, blocked.Close())
	closureMust(t, context.Initialize(rules))
	defer func() {
		closureMust(t, context.Close())
		closureMust(t, rules.Close())
		closureMust(t, ruleManager.Close())
		closureMust(t, transformer.Close())
		closureMust(t, spareOwner.Close())
		closeBase()
	}()
	status, err := context.Solve(query)
	closureMust(t, err)
	if status != lbool.False {
		t.Fatal("certificate converter setup", status)
	}
	oldLevel, err := diagnostic.ProcessVerbosityLevel()
	closureMust(t, err)
	oldStream, err := diagnostic.BorrowProcessVerboseStream()
	closureMust(t, err)
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: manager.Budget()})
	closureMust(t, err)
	defer func() {
		closureMust(t, diagnostic.SetProcessVerbosityLevel(oldLevel))
		closureMust(t, diagnostic.SetProcessVerboseStream(oldStream))
		closureMust(t, stream.Close())
	}()
	closureMust(t, diagnostic.SetProcessVerboseStream(stream))
	closureMust(t, diagnostic.SetProcessVerbosityLevel(0))
	closureMust(t, context.verboseUnreachableCertificate())
	raw, err := stream.String()
	closureMust(t, err)
	if raw != "" || converter.calls != 0 {
		t.Fatalf("disabled verbose certificate evaluated operand: calls=%d output=%q", converter.calls, raw)
	}
	closureMust(t, diagnostic.SetProcessVerbosityLevel(1))
	closureMust(t, context.verboseUnreachableCertificate())
	raw, err = stream.String()
	closureMust(t, err)
	if !strings.Contains(raw, "certificate_preprocessed_predicate") || strings.Contains(raw, "certificate_original_predicate") || converter.calls != 0 {
		t.Fatalf("raw verbose certificate used converter: calls=%d output=%q", converter.calls, raw)
	}
	var converted bytes.Buffer
	closureMust(t, context.DisplayCertificate(&converted, query, false))
	if !strings.Contains(converted.String(), "certificate_original_predicate") || strings.Contains(converted.String(), "certificate_preprocessed_predicate") || converter.calls != 1 {
		t.Fatalf("TRACE certificate omitted converter: calls=%d output=%q", converter.calls, converted.String())
	}
}
