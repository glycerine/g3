package api

import (
	"errors"
	"testing"
)

func TestAPITacticApplyWithParametersClosedOwners(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	goal := apiParameterizedTacticGoal(t, f)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	simplify, err := f.c.Tactic("simplify")
	astTestMust(t, err)
	parameters := apiSimplifyParameters(t, f)
	astTestMust(t, parameters.Close())
	if _, err = simplify.ApplyWithParameters(goal, parameters); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed parameters", err)
	}
	astTestMust(t, simplify.Close())
	parameters = apiSimplifyParameters(t, f)
	t.Cleanup(func() { astTestMust(t, parameters.Close()) })
	if _, err = simplify.ApplyWithParameters(goal, parameters); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed tactic", err)
	}
}
