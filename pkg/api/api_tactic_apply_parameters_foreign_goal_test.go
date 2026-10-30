package api

import (
	"errors"
	"testing"
)

func TestAPITacticApplyWithParametersRejectsForeignGoal(t *testing.T) {
	left := newSMTAPIFixture(t, "ALL", nil)
	right := newSMTAPIFixture(t, "ALL", nil)
	goal := apiParameterizedTacticGoal(t, right)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	simplify, err := left.c.Tactic("simplify")
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, simplify.Close()) })
	parameters := apiSimplifyParameters(t, left)
	t.Cleanup(func() { astTestMust(t, parameters.Close()) })
	if _, err = simplify.ApplyWithParameters(goal, parameters); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("foreign goal", err)
	}
}
