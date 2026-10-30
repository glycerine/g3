package api

import (
	"errors"
	"testing"
)

func TestAPITacticApplyWithParametersRejectsForeignParameters(t *testing.T) {
	left := newSMTAPIFixture(t, "ALL", nil)
	right := newSMTAPIFixture(t, "ALL", nil)
	goal := apiParameterizedTacticGoal(t, left)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	simplify, err := left.c.Tactic("simplify")
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, simplify.Close()) })
	parameters, err := right.c.NewParameterSet()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, parameters.Close()) })
	if _, err = simplify.ApplyWithParameters(goal, parameters); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("foreign parameters", err)
	}
}
