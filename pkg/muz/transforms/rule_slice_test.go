package transforms_test

// Source: src/muz/transforms/dl_mk_slice.cpp predicate-position saturation
// and rule/predicate rewriting.

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func TestRuleSliceRemovesUnconstrainedPredicateColumns(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	zero, err := f.manager.ArithmeticNumeralRat(big.NewRat(0, 1), true)
	coneMust(t, err)
	integer, err := f.manager.GetSort(zero)
	coneMust(t, err)
	declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		symbol, makeErr := f.manager.Symbols.Intern("slice-" + name)
		coneMust(t, makeErr)
		declaration, makeErr := f.manager.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
		coneMust(t, makeErr)
		coneMust(t, f.rules.RegisterPredicate(declaration))
		return declaration
	}
	p := declare("P", []ast.ASTHandle{integer, integer})
	goal := declare("Goal", nil)
	x, err := f.manager.MkVar(0, integer)
	coneMust(t, err)
	y, err := f.manager.MkVar(1, integer)
	coneMust(t, err)
	pXY, err := f.manager.MkApp(p, []ast.ASTHandle{x, y})
	coneMust(t, err)
	goalApp, err := f.manager.MkApp(goal, nil)
	coneMust(t, err)
	add := func(head ast.ASTHandle, tails []ast.ASTHandle, name string) {
		rule, makeErr := f.rules.NewRule(head, tails, nil, name)
		coneMust(t, makeErr)
		coneMust(t, f.set.Add(rule))
		coneMust(t, rule.Close())
	}
	add(pXY, nil, "universal-P")
	add(goalApp, []ast.ASTHandle{pXY}, "goal")
	coneMust(t, f.set.SetOutputPredicate(goal))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("slice fixture rejected")
	}
	plugin := transforms.NewRuleSlicePlugin(f.manager, f.rules, 0)
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	coneMust(t, transformer.Register(plugin))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !modified {
		t.Fatal("slice plugin did not rewrite unconstrained columns")
	}
	derived, found := plugin.DerivedPredicate(p)
	if !found {
		t.Fatal("slice plugin did not publish predicate mapping")
	}
	predicateSnapshot, sliceSnapshot := plugin.PredicateSlices()
	if predicateSnapshot[p] != derived || len(sliceSnapshot[p]) != 2 || !sliceSnapshot[p][0] || !sliceSnapshot[p][1] {
		t.Fatal("slice plugin snapshot does not describe the removed predicate columns", predicateSnapshot, sliceSnapshot)
	}
	sliceSnapshot[p][0] = false
	_, freshSliceSnapshot := plugin.PredicateSlices()
	if !freshSliceSnapshot[p][0] {
		t.Fatal("slice plugin exposed its mutable internal column mask")
	}
	if arity, arityErr := f.manager.GetArity(derived); arityErr != nil || arity != 0 {
		t.Fatal("sliced predicate arity", arity, arityErr)
	}
	if mapped, mapErr := f.set.DerivedPredicate(p); mapErr != nil || mapped != derived {
		t.Fatal("sliced RuleSet mapping", mapped, derived, mapErr)
	}
	if output, outputErr := f.set.OutputPredicate(); outputErr != nil || output != goal {
		t.Fatal("slice output predicate", output, outputErr)
	}
	count, err := f.set.Len()
	coneMust(t, err)
	if count != 2 {
		t.Fatalf("sliced rule count %d, want 2", count)
	}
	for index := 0; index < count; index++ {
		rule, getErr := f.set.Rule(index)
		coneMust(t, getErr)
		head, getErr := rule.HeadDeclaration()
		if getErr == nil && head == derived {
			var application ast.ASTHandle
			application, getErr = rule.Head()
			if getErr == nil {
				var arguments uint32
				arguments, getErr = f.manager.GetNumArgs(application)
				if getErr == nil && arguments != 0 {
					t.Fatalf("sliced head retained %d arguments", arguments)
				}
			}
		}
		getErr = errors.Join(getErr, rule.Close())
		coneMust(t, getErr)
	}
}

func TestRuleSliceLeavesZeroArityOutputRulesUnchanged(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	symbol, err := f.manager.Symbols.Intern("slice-zero-arity-output")
	coneMust(t, err)
	goal, err := f.manager.MkFuncDecl(symbol, nil, boolean, ast.ASTFuncDeclInfo{})
	coneMust(t, err)
	coneMust(t, f.rules.RegisterPredicate(goal))
	application, err := f.manager.MkApp(goal, nil)
	coneMust(t, err)
	rule, err := f.rules.NewRule(application, nil, nil, "zero-arity-output")
	coneMust(t, err)
	coneMust(t, f.set.Add(rule))
	coneMust(t, rule.Close())
	coneMust(t, f.set.SetOutputPredicate(goal))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("zero-arity slice fixture rejected")
	}

	plugin := transforms.NewRuleSlicePlugin(f.manager, f.rules, 0)
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	coneMust(t, transformer.Register(plugin))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if modified {
		t.Fatal("zero-arity-only output rules should not produce a sliced replacement")
	}
	if _, found := plugin.DerivedPredicate(goal); found {
		t.Fatal("zero-arity output acquired a derived predicate")
	}
	count, err := f.set.Len()
	coneMust(t, err)
	if count != 1 {
		t.Fatal("zero-arity source rules changed", count)
	}
}

func TestRuleSliceRetainsConstantConstrainedColumn(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	zero, err := f.manager.ArithmeticNumeralRat(big.NewRat(0, 1), true)
	coneMust(t, err)
	integer, err := f.manager.GetSort(zero)
	coneMust(t, err)
	declare := func(name string, domain []ast.ASTHandle) ast.ASTHandle {
		symbol, makeErr := f.manager.Symbols.Intern("slice-constant-" + name)
		coneMust(t, makeErr)
		declaration, makeErr := f.manager.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
		coneMust(t, makeErr)
		coneMust(t, f.rules.RegisterPredicate(declaration))
		return declaration
	}
	p := declare("P", []ast.ASTHandle{integer, integer})
	goal := declare("Goal", nil)
	x, err := f.manager.MkVar(0, integer)
	coneMust(t, err)
	y, err := f.manager.MkVar(1, integer)
	coneMust(t, err)
	pXY, err := f.manager.MkApp(p, []ast.ASTHandle{x, y})
	coneMust(t, err)
	pZeroY, err := f.manager.MkApp(p, []ast.ASTHandle{zero, y})
	coneMust(t, err)
	goalApp, err := f.manager.MkApp(goal, nil)
	coneMust(t, err)
	for _, spec := range []struct {
		head  ast.ASTHandle
		tails []ast.ASTHandle
		name  string
	}{{pXY, nil, "universal-P"}, {goalApp, []ast.ASTHandle{pZeroY}, "goal"}} {
		rule, makeErr := f.rules.NewRule(spec.head, spec.tails, nil, spec.name)
		coneMust(t, makeErr)
		coneMust(t, f.set.Add(rule))
		coneMust(t, rule.Close())
	}
	coneMust(t, f.set.SetOutputPredicate(goal))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("constant slice fixture rejected")
	}
	plugin := transforms.NewRuleSlicePlugin(f.manager, f.rules, 0)
	transformer := base.NewRuleTransformer()
	t.Cleanup(func() { coneMust(t, transformer.Close()) })
	coneMust(t, transformer.Register(plugin))
	modified, err := transformer.Apply(f.set)
	coneMust(t, err)
	if !modified {
		t.Fatal("constant slice fixture was not rewritten")
	}
	derived, found := plugin.DerivedPredicate(p)
	if !found {
		t.Fatal("constant slice mapping missing")
	}
	if arity, arityErr := f.manager.GetArity(derived); arityErr != nil || arity != 1 {
		t.Fatal("constant-constrained sliced arity", arity, arityErr)
	}
	rules, err := f.set.PredicateRules(goal)
	coneMust(t, err)
	defer func() {
		for _, rule := range rules {
			coneMust(t, rule.Close())
		}
	}()
	if len(rules) != 1 {
		t.Fatalf("constant slice goal rules %d, want 1", len(rules))
	}
	tail, _, err := rules[0].Tail(0)
	coneMust(t, err)
	if declaration, declarationErr := f.manager.GetDecl(tail); declarationErr != nil || declaration != derived {
		t.Fatal("constant slice tail declaration", declaration, derived, declarationErr)
	}
	argument, err := f.manager.GetArg(tail, 0)
	coneMust(t, err)
	if argument != zero {
		t.Fatal("constant slice did not retain constrained argument", argument, zero)
	}
}
