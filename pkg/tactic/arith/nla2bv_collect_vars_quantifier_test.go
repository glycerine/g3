package arith

// Source: src/tactic/arith/nla2bv_tactic.cpp:301-340.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestNLA2BVCollectsVariablesBeforeRejectingQuantifier(t *testing.T) {
	r := nla2BVFixture(t, false)
	m := r.manager
	arithmetic, err := m.ArithmeticFamilyID()
	equationMust(t, err)
	integer, err := m.MkFamilySort(arithmetic, 1, nil)
	equationMust(t, err)
	free := fixDLConst(t, m, true, "quantifier-free-int")
	freeDecl, err := m.GetDecl(free)
	equationMust(t, err)
	bound, err := m.MkVar(0, integer)
	equationMust(t, err)
	equation, err := m.MkEq(free, bound)
	equationMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: true,
		Sorts:  []ast.ASTHandle{integer},
		Names:  []usymbol.Symbol{usymbol.Numerical(0)},
		Body:   equation,
	})
	equationMust(t, err)
	fixDLPin(t, m, quantifier)
	goal := fixDLGoal(t, m, false, false, false, quantifier)
	t.Cleanup(func() { equationMust(t, goal.Close()) })

	inv, err := r.newInvocation()
	equationMust(t, err)
	t.Cleanup(func() { equationMust(t, inv.close()) })
	filter, err := tactic.NewFilterModel(m)
	equationMust(t, err)
	equationMust(t, inv.filterOwner.Prepare(m))
	if err = inv.filterOwner.SetModelConverter(filter); err != nil {
		equationMust(t, filter.Close())
		t.Fatal(err)
	}
	inv.filter = filter
	equationMust(t, inv.bounds.CollectGoal(goal))

	supported, err := inv.collectVars(goal)
	equationMust(t, err)
	if supported {
		t.Fatal("quantified formula accepted by the supported-fragment check")
	}
	if len(inv.vars) != 1 || len(inv.defs) != 1 {
		t.Fatalf("source collects free variables before returning unsupported: vars=%d defs=%d", len(inv.vars), len(inv.defs))
	}
	if got := inv.vars[0]; got != freeDecl {
		t.Fatalf("collected variable declaration = %v, want %v", got, freeDecl)
	}
}
