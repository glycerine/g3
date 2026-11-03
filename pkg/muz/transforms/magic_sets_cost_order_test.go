package transforms_test

// Source: src/muz/transforms/dl_mk_magic_sets.cpp get_unbound_cost and
// pop_bound select the eligible tail with the smallest unbound-domain product,
// not the tail with the most already-bound argument positions.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

func TestMagicSetsOrdersEligibleTailsByUnboundDomainCost(t *testing.T) {
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	bits, err := f.manager.BitVectorSort(8)
	coneMust(t, err)
	x, err := f.manager.MkVar(0, boolean)
	coneMust(t, err)
	w, err := f.manager.MkVar(1, boolean)
	coneMust(t, err)
	z, err := f.manager.MkVar(2, bits)
	coneMust(t, err)
	no, err := f.manager.MkFalse()
	coneMust(t, err)

	declare := func(name string, domain, arguments []ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle) {
		t.Helper()
		symbol, declarationErr := f.manager.Symbols.Intern("magic-cost-" + name)
		coneMust(t, declarationErr)
		declaration, declarationErr := f.manager.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
		coneMust(t, declarationErr)
		coneMust(t, f.rules.RegisterPredicate(declaration))
		application, declarationErr := f.manager.MkApp(declaration, arguments)
		coneMust(t, declarationErr)
		return declaration, application
	}

	aDecl, a := declare("A", []ast.ASTHandle{boolean, boolean, bits}, []ast.ASTHandle{x, x, z})
	bDecl, b := declare("B", []ast.ASTHandle{boolean, boolean}, []ast.ASTHandle{x, w})
	qDecl, q := declare("Q", []ast.ASTHandle{boolean}, []ast.ASTHandle{x})
	goalDecl, goal := declare("Goal", nil, nil)
	qAtFalse, err := f.manager.MkApp(qDecl, []ast.ASTHandle{no})
	coneMust(t, err)

	addTransformRule(t, f, q, []ast.ASTHandle{a, b}, nil, "magic-cost-q-rule")
	addTransformRule(t, f, goal, []ast.ASTHandle{qAtFalse}, nil, "magic-cost-goal-rule")
	coneMust(t, f.set.SetOutputPredicate(goalDecl))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("magic cost fixture rejected")
	}

	plugin := transforms.NewMagicSetsPlugin(f.manager, f.rules, goalDecl, true, 0)
	defer func() { coneMust(t, plugin.Close()) }()
	result, err := plugin.Transform(f.set)
	coneMust(t, err)
	if result == nil {
		t.Fatal("magic cost transform returned nil")
	}
	defer func() { coneMust(t, result.Close()) }()

	count, err := result.Len()
	coneMust(t, err)
	found := 0
	for index := 0; index < count; index++ {
		rule, getErr := result.Rule(index)
		coneMust(t, getErr)
		name, getErr := rule.Name()
		if getErr == nil && name == "magic-cost-q-rule" {
			found++
			tails, tailErr := rule.TailSize()
			coneMust(t, tailErr)
			if tails != 3 {
				t.Fatal("transformed Q tail count", tails)
			}
			first, tailErr := rule.TailDeclaration(0)
			coneMust(t, tailErr)
			second, tailErr := rule.TailDeclaration(1)
			coneMust(t, tailErr)
			if first != bDecl || second != aDecl {
				t.Fatalf("tail order = %v,%v, want B,A", first, second)
			}
		}
		coneMust(t, getErr)
		coneMust(t, rule.Close())
	}
	if found != 1 {
		t.Fatal("transformed named Q rule count", found)
	}
}

func TestMagicSetsUnboundCostUsesSourceCardinalityFallbacks(t *testing.T) {
	t.Run("rule-sort-is-one", func(t *testing.T) {
		checkMagicSetsLowerCostSecondTail(t, func(f *coneFixture) (ast.ASTHandle, ast.ASTHandle) {
			boolean, err := f.manager.MkBoolSort()
			coneMust(t, err)
			ruleSort, err := f.manager.MkDatalogRuleSort()
			coneMust(t, err)
			return boolean, ruleSort
		})
	})
	t.Run("infinite-sort-is-maximum", func(t *testing.T) {
		checkMagicSetsLowerCostSecondTail(t, func(f *coneFixture) (ast.ASTHandle, ast.ASTHandle) {
			name, err := f.manager.Symbols.Intern("magic-cost-infinite-domain")
			coneMust(t, err)
			infinite, err := f.manager.MkUninterpretedSort(name, nil)
			coneMust(t, err)
			finite, err := f.manager.BitVectorSort(8)
			coneMust(t, err)
			return infinite, finite
		})
	})
}

func checkMagicSetsLowerCostSecondTail(t *testing.T, sorts func(*coneFixture) (ast.ASTHandle, ast.ASTHandle)) {
	t.Helper()
	f := newConeFixture(t)
	boolean, err := f.manager.MkBoolSort()
	coneMust(t, err)
	firstSort, secondSort := sorts(f)
	x, err := f.manager.MkVar(0, boolean)
	coneMust(t, err)
	firstVariable, err := f.manager.MkVar(1, firstSort)
	coneMust(t, err)
	secondVariable, err := f.manager.MkVar(2, secondSort)
	coneMust(t, err)
	no, err := f.manager.MkFalse()
	coneMust(t, err)

	declare := func(name string, domain, arguments []ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle) {
		t.Helper()
		symbol, declarationErr := f.manager.Symbols.Intern("magic-cost-fallback-" + name)
		coneMust(t, declarationErr)
		declaration, declarationErr := f.manager.MkFuncDecl(symbol, domain, boolean, ast.ASTFuncDeclInfo{})
		coneMust(t, declarationErr)
		coneMust(t, f.rules.RegisterPredicate(declaration))
		application, declarationErr := f.manager.MkApp(declaration, arguments)
		coneMust(t, declarationErr)
		return declaration, application
	}

	firstDecl, first := declare("first", []ast.ASTHandle{boolean, firstSort}, []ast.ASTHandle{x, firstVariable})
	secondDecl, second := declare("second", []ast.ASTHandle{boolean, secondSort}, []ast.ASTHandle{x, secondVariable})
	qDecl, q := declare("Q", []ast.ASTHandle{boolean}, []ast.ASTHandle{x})
	goalDecl, goal := declare("Goal", nil, nil)
	qAtFalse, err := f.manager.MkApp(qDecl, []ast.ASTHandle{no})
	coneMust(t, err)
	addTransformRule(t, f, q, []ast.ASTHandle{first, second}, nil, "magic-cost-fallback-rule")
	addTransformRule(t, f, goal, []ast.ASTHandle{qAtFalse}, nil, "magic-cost-fallback-goal")
	coneMust(t, f.set.SetOutputPredicate(goalDecl))
	valid, err := f.set.Finalize()
	coneMust(t, err)
	if !valid {
		t.Fatal("magic fallback fixture rejected")
	}

	plugin := transforms.NewMagicSetsPlugin(f.manager, f.rules, goalDecl, true, 0)
	defer func() { coneMust(t, plugin.Close()) }()
	result, err := plugin.Transform(f.set)
	coneMust(t, err)
	if result == nil {
		t.Fatal("magic fallback transform returned nil")
	}
	defer func() { coneMust(t, result.Close()) }()
	count, err := result.Len()
	coneMust(t, err)
	found := 0
	for index := 0; index < count; index++ {
		rule, getErr := result.Rule(index)
		coneMust(t, getErr)
		name, getErr := rule.Name()
		if getErr == nil && name == "magic-cost-fallback-rule" {
			found++
			firstTail, tailErr := rule.TailDeclaration(0)
			coneMust(t, tailErr)
			secondTail, tailErr := rule.TailDeclaration(1)
			coneMust(t, tailErr)
			if firstTail != secondDecl || secondTail != firstDecl {
				t.Fatalf("fallback tail order = %v,%v, want lower-cost second then first", firstTail, secondTail)
			}
		}
		coneMust(t, getErr)
		coneMust(t, rule.Close())
	}
	if found != 1 {
		t.Fatal("transformed fallback rule count", found)
	}
}
