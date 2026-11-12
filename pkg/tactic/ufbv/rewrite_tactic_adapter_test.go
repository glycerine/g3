package ufbv

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestUFBVRewriterCommonTacticAdapterBalancesGoalReference(t *testing.T) {
	m := cacheFixture(t)
	context, globals := ufbvTacticServicesFixture(t, m)
	runner, err := NewRewriterTacticAdapter(m, context, globals)
	cacheMust(t, err)
	t.Cleanup(func() { cacheMust(t, runner.Close()) })
	goal, err := tactic.NewGoal(m, false, false, false)
	cacheMust(t, err)
	t.Cleanup(func() { cacheMust(t, goal.Close()) })
	before := goal.RefCount
	var result tactic.TacticResult
	t.Cleanup(func() { cacheMust(t, result.Close()) })
	cacheMust(t, runner.Apply(goal, &result))
	// The result borrows the caller's goal: no extra reference survives, and
	// releasing the result must not close the goal (the "arena: closed"
	// failure of the ufbv/bv tactics under repeat).
	if len(result.Goals) != 1 || result.Goals[0] != goal || goal.RefCount != before {
		t.Fatal("common adapter goal ownership", len(result.Goals), result.Goals[0], goal.RefCount, before)
	}
	cacheMust(t, result.Reset())
	if goal.Closed {
		t.Fatal("releasing the adapter result closed the caller's goal")
	}
	if _, err = goal.Size(); err != nil {
		t.Fatal(err)
	}
}

func TestUFBVRewriterAdapterRetainsQuantifiedBitVectorResult(t *testing.T) {
	m, services := macroTacticFixture(t, false)
	_, err := m.RegisterArithmeticPlugin()
	cacheMust(t, err)
	_, err = m.RegisterBitVectorPlugin()
	cacheMust(t, err)
	sort, err := m.BitVectorSort(4)
	cacheMust(t, err)
	variable, err := m.MkVar(0, sort)
	cacheMust(t, err)
	zero, err := m.BitVectorNumeralUnsigned(0, 4)
	cacheMust(t, err)
	body, err := m.MkEq(variable, zero)
	cacheMust(t, err)
	quantifier, err := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{sort}, Names: []usymbol.Symbol{usymbol.Numerical(7100)}, Body: body})
	cacheMust(t, err)
	goal, err := tactic.NewGoal(m, false, false, false)
	cacheMust(t, err)
	t.Cleanup(func() { cacheMust(t, goal.Close()) })
	cacheMust(t, goal.Assert(quantifier))
	runner, err := NewRewriterTacticAdapter(m, services.Context, services.Globals)
	cacheMust(t, err)
	t.Cleanup(func() { cacheMust(t, runner.Close()) })
	var result tactic.TacticResult
	t.Cleanup(func() { cacheMust(t, result.Close()) })
	cacheMust(t, runner.Apply(goal, &result))
	form, err := goal.Form(0)
	cacheMust(t, err)
	if _, err = m.GetKind(form); err != nil {
		t.Fatal("rewriter published dead quantified BV result", err)
	}
}
