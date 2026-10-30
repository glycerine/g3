package api

import (
	"errors"
	"testing"
)

func TestAPIOptimizationObjectivesOwnResultsAndGuardClosedContext(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_LIA", nil)
	x := f.arithmeticVariable("objective-lifecycle-x", true)
	astTestMust(t, f.c.Push())
	astTestMust(t, f.c.Maximize(x))
	objectives, err := f.c.OptimizationObjectives()
	astTestMust(t, err)
	astTestMust(t, f.c.Pop(1))
	if count, countErr := f.c.OptimizationObjectiveCount(); countErr != nil || count != 0 {
		t.Fatal("reset count", count, countErr)
	}
	handle, err := f.m.RefGet(objectives[0])
	astTestMust(t, err)
	if handle.IsNull() {
		t.Fatal("owned objective retired")
	}
	astTestMust(t, f.m.DelRef(&objectives[0]))
	closed, err := NewSMTContext()
	astTestMust(t, err)
	astTestMust(t, closed.Close())
	if _, err = closed.OptimizationObjectiveCount(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed count", err)
	}
	if _, err = closed.OptimizationObjectives(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed objectives", err)
	}
}
