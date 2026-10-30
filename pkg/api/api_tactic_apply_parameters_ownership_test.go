package api

import "testing"

func TestAPITacticApplyWithParametersResultOwnsOutput(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	goal := apiParameterizedTacticGoal(t, f)
	simplify, err := f.c.Tactic("simplify")
	astTestMust(t, err)
	parameters := apiSimplifyParameters(t, f)
	result, err := simplify.ApplyWithParameters(goal, parameters)
	astTestMust(t, err)
	astTestMust(t, goal.Close())
	astTestMust(t, parameters.Close())
	astTestMust(t, simplify.Close())
	if count, countErr := result.NumSubgoals(); countErr != nil || count != 1 {
		t.Fatal("owned result", count, countErr)
	}
	astTestMust(t, result.Close())
}
