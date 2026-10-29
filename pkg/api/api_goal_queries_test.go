package api

import (
	"testing"
)

func TestAPIGoalQueriesAndFormulaOwnership(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	goal, err := f.c.NewGoal(false, false, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	boolean := f.pin(f.c.BoolSort())
	x := f.pin(f.c.Constant("goal-query-x", boolean))
	notX := f.pin(f.c.Not(x))
	conjunction := f.pin(f.c.And(x, notX))
	astTestMust(t, goal.Assert(conjunction))
	if inconsistent, e := goal.Inconsistent(); e != nil || inconsistent {
		t.Fatal("syntactically non-false goal", inconsistent, e)
	}
	if count, e := goal.NumExpressions(); e != nil || count < 2 {
		t.Fatal("expression count", count, e)
	}
	formulaRef, err := goal.Formula(0)
	astTestMust(t, err)
	defer func() { astTestMust(t, f.m.DelRef(&formulaRef)) }()
	formula, err := f.m.RefGet(formulaRef)
	astTestMust(t, err)
	if sort, e := f.m.GetSort(formula); e != nil || sort != boolean {
		t.Fatal("selected formula sort", sort, boolean, e)
	}
	astTestMust(t, goal.Reset())
	if _, err = f.m.GetKind(formula); err != nil {
		t.Fatal("owning formula did not survive reset", err)
	}

	falseGoal, err := f.c.NewGoal(false, false, false)
	astTestMust(t, err)
	defer func() { astTestMust(t, falseGoal.Close()) }()
	falseValue := f.pin(f.c.Bool(false))
	astTestMust(t, falseGoal.Assert(falseValue))
	if unsat, e := falseGoal.IsDecidedUnsat(); e != nil || !unsat {
		t.Fatal("decided unsat", unsat, e)
	}
	empty, err := f.c.NewGoal(false, false, false)
	astTestMust(t, err)
	defer func() { astTestMust(t, empty.Close()) }()
	if sat, e := empty.IsDecidedSat(); e != nil || !sat {
		t.Fatal("empty decided sat", sat, e)
	}
}
