package api

import (
	"errors"
	"testing"
)

func TestAPIGoalDIMACSNamesAndClosedOwner(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	goal, err := f.c.NewGoal(false, false, false)
	astTestMust(t, err)
	boolean := f.pin(f.c.BoolSort())
	p := f.pin(f.c.Constant("api-named-p", boolean))
	astTestMust(t, goal.Assert(p))
	text, err := goal.DIMACS(true)
	astTestMust(t, err)
	if want := "p cnf 1 1\n1 0\nc 1 api-named-p"; text != want {
		t.Fatalf("named DIMACS = %q; want %q", text, want)
	}
	astTestMust(t, goal.Close())
	if text, err = goal.DIMACS(false); text != "" || !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed DIMACS export", text, err)
	}
}
