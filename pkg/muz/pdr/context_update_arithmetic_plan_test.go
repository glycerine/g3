package pdr

// Source: context::update_rules and init_core_generalizers in
// src/muz/pdr/pdr_context.cpp.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestContextUpdateRulesRejectsStaleArithmeticFragment(t *testing.T) {
	manager, oldOwner, transformer, _, closeBase := closureFixture(t)
	config := oldOwner.contexts.config
	closureMust(t, transformer.Close())
	closureMust(t, oldOwner.Close())
	defer closeBase()

	context, err := NewContext(manager, nil, 2, ReachableNoCache, config)
	closureMust(t, err)
	defer func() { closureMust(t, context.Close()) }()
	closureMust(t, context.SetUseUTVPI(true))

	arithmetic, err := manager.ArithmeticFamilyID()
	closureMust(t, err)
	realSort, err := manager.MkFamilySort(arithmetic, 0, nil)
	closureMust(t, err)
	boolean, err := manager.MkBoolSort()
	closureMust(t, err)
	name, err := manager.Symbols.Intern("update-arithmetic-fragment")
	closureMust(t, err)
	declaration, err := manager.MkFuncDecl(name, []ast.ASTHandle{realSort, realSort, realSort}, boolean, ast.ASTFuncDeclInfo{})
	closureMust(t, err)
	rules, err := base.NewRuleManager(manager)
	closureMust(t, err)
	defer func() { closureMust(t, rules.Close()) }()
	closureMust(t, rules.RegisterPredicate(declaration))

	x, err := manager.MkVar(0, realSort)
	closureMust(t, err)
	y, err := manager.MkVar(1, realSort)
	closureMust(t, err)
	z, err := manager.MkVar(2, realSort)
	closureMust(t, err)
	head, err := manager.MkApp(declaration, []ast.ASTHandle{x, y, z})
	closureMust(t, err)
	zero, err := manager.ArithmeticRealNumeralText("0")
	closureMust(t, err)
	minusOne, err := manager.ArithmeticRealNumeralText("-1")
	closureMust(t, err)
	negativeY, err := manager.MkArithmeticMul(minusOne, y)
	closureMust(t, err)
	difference, err := manager.MkArithmeticAdd(x, negativeY)
	closureMust(t, err)
	differenceBound, err := manager.MkArithmeticLE(difference, zero)
	closureMust(t, err)
	sum, err := manager.MkArithmeticAdd(x, y)
	closureMust(t, err)
	threeWay, err := manager.MkArithmeticAdd(sum, z)
	closureMust(t, err)
	generalBound, err := manager.MkArithmeticLE(threeWay, zero)
	closureMust(t, err)

	makeSet := func(label string, bound ast.ASTHandle) *base.RuleSet {
		set, xerr := base.NewRuleSet(rules)
		closureMust(t, xerr)
		rule, xerr := rules.NewRule(head, []ast.ASTHandle{bound}, nil, label)
		closureMust(t, xerr)
		closureMust(t, set.Add(rule))
		closureMust(t, rule.Close())
		return set
	}
	differenceSet := makeSet("update-difference", differenceBound)
	defer func() { closureMust(t, differenceSet.Close()) }()
	generalSet := makeSet("update-general", generalBound)
	defer func() { closureMust(t, generalSet.Close()) }()

	closureMust(t, context.Initialize(differenceSet))
	if difference, xerr := context.IsDifferenceLogic(); xerr != nil || !difference {
		t.Fatal("initial difference-logic selection", difference, xerr)
	}
	if compatible, xerr := context.CanUpdateRules(differenceSet); xerr != nil || !compatible {
		t.Fatal("same-fragment update compatibility", compatible, xerr)
	}
	if compatible, xerr := context.CanUpdateRules(generalSet); xerr != nil || compatible {
		t.Fatal("cross-fragment update compatibility", compatible, xerr)
	}
	if xerr := context.UpdateRules(generalSet); !errors.Is(xerr, ast.ErrASTDomain) {
		t.Fatal("cross-fragment in-place update", xerr)
	}
	if difference, xerr := context.IsDifferenceLogic(); xerr != nil || !difference {
		t.Fatal("rejected update changed arithmetic fragment", difference, xerr)
	}
	if status, xerr := context.Solve(declaration); xerr != nil || status != lbool.True {
		t.Fatal("rejected update damaged old program", status, xerr)
	}

	closureMust(t, context.Reset())
	closureMust(t, context.Initialize(generalSet))
	if difference, xerr := context.IsDifferenceLogic(); xerr != nil || difference {
		t.Fatal("rebuilt general-arithmetic selection", difference, xerr)
	}
	if utvpi, xerr := context.IsUTVPI(); xerr != nil || utvpi {
		t.Fatal("rebuilt general-arithmetic UTVPI selection", utvpi, xerr)
	}
	if status, xerr := context.Solve(declaration); xerr != nil || status != lbool.True {
		t.Fatal("rebuilt general-arithmetic program", status, xerr)
	}
}
