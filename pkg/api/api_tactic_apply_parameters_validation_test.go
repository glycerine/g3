package api

import "testing"

func TestAPITacticApplyWithParametersRejectsUnknownOption(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	goal := apiParameterizedTacticGoal(t, f)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	simplify, err := f.c.Tactic("simplify")
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, simplify.Close()) })
	parameters, err := f.c.NewParameterSet()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, parameters.Close()) })
	astTestMust(t, parameters.SetBool("not-a-simplifier-option", true))
	if result, applyErr := simplify.ApplyWithParameters(goal, parameters); applyErr == nil || result != nil {
		t.Fatal("unknown parameter accepted", result, applyErr)
	}
}
