package fp

// Source: pdr::context::update_rules and init_core_generalizers in
// src/muz/pdr/pdr_context.cpp, reached through pdr_dl_interface::query.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestPDREngineRebuildsContextAcrossArithmeticFragments(t *testing.T) {
	manager, globals := newPDRLifecycleServices(t)
	engine, err := NewPropositionalEngineWithServices(manager, globals.Context, globals)
	fragmentMust(t, err)
	defer func() { fragmentMust(t, engine.Close()) }()
	fragmentMust(t, engine.SelectEngine("pdr"))
	parameters, err := engine.params.NewValue()
	fragmentMust(t, err)
	defer func() { fragmentMust(t, engine.params.Del(&parameters)) }()
	for _, parameter := range []string{"xform.slice", "xform.inline_eager", "xform.inline_linear"} {
		fragmentMust(t, engine.params.SetBoolName(&parameters, &parameter, false))
	}
	fragmentMust(t, engine.SetQueryParameters(&parameters))

	arithmetic, err := manager.RegisterArithmeticPlugin()
	fragmentMust(t, err)
	realSort, err := manager.MkFamilySort(arithmetic, 0, nil)
	fragmentMust(t, err)
	boolean, err := manager.MkBoolSort()
	fragmentMust(t, err)
	name, err := manager.Symbols.Intern("pdr-fragment-rebuild")
	fragmentMust(t, err)
	predicate, err := manager.MkFuncDecl(name, []ast.ASTHandle{realSort, realSort}, boolean, ast.ASTFuncDeclInfo{})
	fragmentMust(t, err)
	fragmentMust(t, engine.RegisterPredicate(predicate))
	x, err := manager.MkVar(0, realSort)
	fragmentMust(t, err)
	y, err := manager.MkVar(1, realSort)
	fragmentMust(t, err)
	head, err := manager.MkApp(predicate, []ast.ASTHandle{x, y})
	fragmentMust(t, err)
	zero, err := manager.ArithmeticRealNumeralText("0")
	fragmentMust(t, err)
	minusOne, err := manager.ArithmeticRealNumeralText("-1")
	fragmentMust(t, err)
	negativeY, err := manager.MkArithmeticMul(minusOne, y)
	fragmentMust(t, err)
	difference, err := manager.MkArithmeticAdd(x, negativeY)
	fragmentMust(t, err)
	differenceBound, err := manager.MkArithmeticLE(difference, zero)
	fragmentMust(t, err)
	differenceRule, err := manager.MkImplies(differenceBound, head)
	fragmentMust(t, err)
	fragmentMust(t, engine.addRuleEager(differenceRule, "pdr-fragment-rule"))

	status, err := engine.QueryPredicate(predicate)
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("difference-logic query", status)
	}
	first := engine.pdr
	if differenceMode, xerr := first.IsDifferenceLogic(); xerr != nil || !differenceMode {
		t.Fatal("initial PDR arithmetic fragment", differenceMode, xerr)
	}
	oldTransformer, found, err := first.Transformer(engine.pdrQueryPredicate(predicate))
	fragmentMust(t, err)
	if !found {
		t.Fatal("initial PDR transformer missing")
	}

	generalProduct, err := manager.MkArithmeticMul(x, y)
	fragmentMust(t, err)
	generalBound, err := manager.MkArithmeticLE(generalProduct, zero)
	fragmentMust(t, err)
	strongerBody, err := manager.MkAnd(differenceBound, generalBound)
	fragmentMust(t, err)
	generalRule, err := manager.MkImplies(strongerBody, head)
	fragmentMust(t, err)
	fragmentMust(t, engine.UpdateRule(generalRule, "pdr-fragment-rule"))

	status, err = engine.QueryPredicate(predicate)
	fragmentMust(t, err)
	if status != lbool.True {
		t.Fatal("general-arithmetic query", status)
	}
	if engine.pdr != first {
		t.Fatal("cross-fragment update replaced instead of resetting the PDR context")
	}
	if _, xerr := oldTransformer.Head(); xerr == nil {
		t.Fatal("cross-fragment update preserved the stale transformer")
	}
	if differenceMode, xerr := engine.pdr.IsDifferenceLogic(); xerr != nil || differenceMode {
		t.Fatal("rebuilt context remained in difference logic", differenceMode, xerr)
	}
	if utvpiMode, xerr := engine.pdr.IsUTVPI(); xerr != nil || utvpiMode {
		t.Fatal("rebuilt context selected UTVPI for general arithmetic", utvpiMode, xerr)
	}
}
