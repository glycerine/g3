package api

import (
	"errors"
	"testing"
)

func TestAPIGoalLifecycleAndConfiguration(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	goal, err := f.c.NewGoal(true, true, false)
	astTestMust(t, err)
	if precision, e := goal.Precision(); e != nil || precision != APIGoalPrecise {
		t.Fatal("initial precision", precision, e)
	}
	if depth, e := goal.Depth(); e != nil || depth != 0 {
		t.Fatal("initial depth", depth, e)
	}
	astTestMust(t, goal.Close())
	if _, err = goal.Size(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed goal", err)
	}
	astTestMust(t, goal.Close())
}
