package api

import "testing"

func TestAPITacticApplyWithParametersPersistsTacticUpdate(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	goal := apiParameterizedTacticGoal(t, f)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	simplify, err := f.c.Tactic("simplify")
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, simplify.Close()) })
	parameters := apiSimplifyParameters(t, f)
	first, err := simplify.ApplyWithParameters(goal, parameters)
	astTestMust(t, err)
	astTestMust(t, first.Close())
	astTestMust(t, parameters.Close())
	second, err := simplify.Apply(goal)
	astTestMust(t, err)
	if count, countErr := second.NumSubgoals(); countErr != nil || count != 1 {
		t.Fatal("persisted apply", count, countErr)
	}
	astTestMust(t, second.Close())
}
