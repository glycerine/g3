package api

import "testing"

func TestAPITacticApplyWithParametersConcreteSuccess(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	goal := apiParameterizedTacticGoal(t, f)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	simplify, err := f.c.Tactic("simplify")
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, simplify.Close()) })
	parameters := apiSimplifyParameters(t, f)
	t.Cleanup(func() { astTestMust(t, parameters.Close()) })
	result, err := simplify.ApplyWithParameters(goal, parameters)
	astTestMust(t, err)
	if count, countErr := result.NumSubgoals(); countErr != nil || count != 1 {
		t.Fatal("subgoals", count, countErr)
	}
	formula := apiParameterizedResultFormula(t, result)
	astTestMust(t, f.m.DelRef(&formula))
	astTestMust(t, result.Close())
}
