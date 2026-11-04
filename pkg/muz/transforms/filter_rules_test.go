package transforms_test

// Source: src/muz/transforms/dl_mk_filter_rules.cpp/.h.

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func TestFilterRulesCachesNormalizedRepeatedArgumentFilters(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	zero, err := f.manager.ArithmeticNumeralRat(big.NewRat(0, 1), true)
	coneMust(t, err)
	integer, err := f.manager.GetSort(zero)
	coneMust(t, err)
	pDecl := separateDeclaration(t, f, "filter-P", []ast.ASTHandle{integer}, boolean)
	rDecl := separateDeclaration(t, f, "filter-R", []ast.ASTHandle{integer}, boolean)
	qDecl := separateDeclaration(t, f, "filter-Q", []ast.ASTHandle{integer, integer}, boolean)
	x, err := f.manager.MkVar(0, integer)
	coneMust(t, err)
	qxx, err := f.manager.MkApp(qDecl, []ast.ASTHandle{x, x})
	coneMust(t, err)
	originals := make(map[ast.ASTHandle]*base.Rule)
	for _, specification := range []struct {
		declaration ast.ASTHandle
		name        string
	}{{pDecl, "P"}, {rDecl, "R"}} {
		head, makeErr := f.manager.MkApp(specification.declaration, []ast.ASTHandle{x})
		coneMust(t, makeErr)
		rule, makeErr := f.rules.NewRule(head, []ast.ASTHandle{qxx}, nil, specification.name)
		coneMust(t, makeErr)
		originals[specification.declaration], makeErr = rule.Retain()
		coneMust(t, makeErr)
		coneMust(t, f.set.Add(rule))
		coneMust(t, rule.Close())
		coneMust(t, f.set.SetOutputPredicate(specification.declaration))
	}
	t.Cleanup(func() {
		for _, original := range originals {
			coneMust(t, original.Close())
		}
	})
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("filter source rules are not stratified")
	}
	plugin := transforms.NewFilterRulesPlugin(f.manager, f.rules, 0)
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	coneMust(t, transformer.Register(plugin))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !modified {
		t.Fatal("repeated predicate argument was not filtered")
	}
	count, err := f.set.Len()
	coneMust(t, err)
	if count != 3 {
		t.Fatalf("filter cache produced %d rules, want two parents and one definition", count)
	}
	var filter ast.ASTHandle
	parents := 0
	definitions := 0
	for index := 0; index < count; index++ {
		rule, getErr := f.set.Rule(index)
		coneMust(t, getErr)
		headDeclaration, getErr := rule.HeadDeclaration()
		if getErr == nil && (headDeclaration == pDecl || headDeclaration == rDecl) {
			checkTransformAccounting(t, rule, originals[headDeclaration], base.Costs{Milliseconds: 7, Instructions: 3})
			parents++
			var tailDeclaration ast.ASTHandle
			tailDeclaration, getErr = rule.TailDeclaration(0)
			if getErr == nil {
				if filter.IsNull() {
					filter = tailDeclaration
				} else if filter != tailDeclaration {
					t.Fatalf("equivalent normalized tails used different filters %v and %v", filter, tailDeclaration)
				}
			}
		} else if getErr == nil {
			checkTransformAccounting(t, rule, originals[pDecl], base.Costs{Milliseconds: 7, Instructions: 3})
			definitions++
			filter = headDeclaration
			var tailDeclaration ast.ASTHandle
			tailDeclaration, getErr = rule.TailDeclaration(0)
			if getErr == nil && tailDeclaration != qDecl {
				t.Fatal("filter definition tail", tailDeclaration, qDecl)
			}
		}
		getErr = errors.Join(getErr, rule.Close())
		coneMust(t, getErr)
	}
	if parents != 2 || definitions != 1 {
		t.Fatal("filter rule classes", parents, definitions)
	}
	if arity, arityErr := f.manager.GetArity(filter); arityErr != nil || arity != 1 {
		t.Fatal("shared repeated variable filter arity", arity, arityErr)
	}
	outputs, err := f.set.OutputPredicates()
	coneMust(t, err)
	if len(outputs) != 2 {
		t.Fatal("outputs not inherited", outputs)
	}
}

func TestFilterRulesLocalRepeatedAndValueArguments(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	zero, err := f.manager.ArithmeticNumeralRat(big.NewRat(0, 1), true)
	coneMust(t, err)
	integer, err := f.manager.GetSort(zero)
	coneMust(t, err)
	pDecl := separateDeclaration(t, f, "filter-local-P", []ast.ASTHandle{integer}, boolean)
	qDecl := separateDeclaration(t, f, "filter-local-Q", []ast.ASTHandle{integer, integer}, boolean)
	sDecl := separateDeclaration(t, f, "filter-value-S", []ast.ASTHandle{integer, integer}, boolean)
	x, err := f.manager.MkVar(0, integer)
	coneMust(t, err)
	y, err := f.manager.MkVar(1, integer)
	coneMust(t, err)
	head, err := f.manager.MkApp(pDecl, []ast.ASTHandle{x})
	coneMust(t, err)
	qyy, err := f.manager.MkApp(qDecl, []ast.ASTHandle{y, y})
	coneMust(t, err)
	sx0, err := f.manager.MkApp(sDecl, []ast.ASTHandle{x, zero})
	coneMust(t, err)
	rule, err := f.rules.NewRule(head, []ast.ASTHandle{qyy, sx0}, nil, "local-and-value")
	coneMust(t, err)
	coneMust(t, f.set.Add(rule))
	coneMust(t, rule.Close())
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("filter source rule is not stratified")
	}
	plugin := transforms.NewFilterRulesPlugin(f.manager, f.rules, 0)
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	coneMust(t, transformer.Register(plugin))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !modified {
		t.Fatal("local repeated/value arguments were not filtered")
	}
	count, err := f.set.Len()
	coneMust(t, err)
	if count != 3 {
		t.Fatalf("rewritten plus two definitions gave %d rules", count)
	}
	arities := map[uint32]bool{}
	for index := 0; index < count; index++ {
		current, getErr := f.set.Rule(index)
		coneMust(t, getErr)
		declaration, getErr := current.HeadDeclaration()
		if getErr == nil && declaration != pDecl {
			var arity uint32
			arity, getErr = f.manager.GetArity(declaration)
			arities[arity] = true
		}
		getErr = errors.Join(getErr, current.Close())
		coneMust(t, getErr)
	}
	if !arities[0] || !arities[1] || len(arities) != 2 {
		t.Fatal("filter domains should distinguish local and shared variables", arities)
	}
}

func TestFilterRulesRemoveDuplicateRewrittenTails(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	zero, err := f.manager.ArithmeticNumeralRat(big.NewRat(0, 1), true)
	coneMust(t, err)
	integer, err := f.manager.GetSort(zero)
	coneMust(t, err)
	pDecl := separateDeclaration(t, f, "filter-duplicate-P", []ast.ASTHandle{integer}, boolean)
	qDecl := separateDeclaration(t, f, "filter-duplicate-Q", []ast.ASTHandle{integer, integer}, boolean)
	x, err := f.manager.MkVar(0, integer)
	coneMust(t, err)
	head, err := f.manager.MkApp(pDecl, []ast.ASTHandle{x})
	coneMust(t, err)
	qxx, err := f.manager.MkApp(qDecl, []ast.ASTHandle{x, x})
	coneMust(t, err)
	rule, err := f.rules.NewRule(head, []ast.ASTHandle{qxx, qxx}, nil, "duplicate")
	coneMust(t, err)
	coneMust(t, f.set.Add(rule))
	coneMust(t, rule.Close())
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("duplicate filter source rule is not stratified")
	}
	plugin := transforms.NewFilterRulesPlugin(f.manager, f.rules, 0)
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	coneMust(t, transformer.Register(plugin))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !modified {
		t.Fatal("duplicate repeated arguments were not filtered")
	}
	parents, err := f.set.PredicateRules(pDecl)
	coneMust(t, err)
	if len(parents) != 1 {
		t.Fatal("rewritten parent count", len(parents))
	}
	tailCount, err := parents[0].TailSize()
	err = errors.Join(err, parents[0].Close())
	coneMust(t, err)
	if tailCount != 1 {
		t.Fatalf("duplicate rewritten tails retained: %d", tailCount)
	}
}

func TestFilterRulesIgnoreDistinctAndNegatedPredicateArguments(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	zero, err := f.manager.ArithmeticNumeralRat(big.NewRat(0, 1), true)
	coneMust(t, err)
	integer, err := f.manager.GetSort(zero)
	coneMust(t, err)
	pDecl := separateDeclaration(t, f, "filter-none-P", []ast.ASTHandle{integer, integer}, boolean)
	qDecl := separateDeclaration(t, f, "filter-none-Q", []ast.ASTHandle{integer, integer}, boolean)
	x, err := f.manager.MkVar(0, integer)
	coneMust(t, err)
	y, err := f.manager.MkVar(1, integer)
	coneMust(t, err)
	head, err := f.manager.MkApp(pDecl, []ast.ASTHandle{x, y})
	coneMust(t, err)
	qxy, err := f.manager.MkApp(qDecl, []ast.ASTHandle{x, y})
	coneMust(t, err)
	qxx, err := f.manager.MkApp(qDecl, []ast.ASTHandle{x, x})
	coneMust(t, err)
	rule, err := f.rules.NewRule(head, []ast.ASTHandle{qxy, qxx}, []bool{false, true}, "unchanged")
	coneMust(t, err)
	coneMust(t, f.set.Add(rule))
	coneMust(t, rule.Close())
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("filter source rule is not stratified")
	}
	plugin := transforms.NewFilterRulesPlugin(f.manager, f.rules, 0)
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	coneMust(t, transformer.Register(plugin))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if modified {
		t.Fatal("distinct positive and repeated negative arguments should be unchanged")
	}
}
