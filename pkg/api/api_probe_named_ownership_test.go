package api

import (
	"errors"
	"testing"
)

func TestAPINamedProbeOwnerAndLifecycle(t *testing.T) {
	left := newSMTAPIFixture(t, "ALL", nil)
	right := newSMTAPIFixture(t, "ALL", nil)
	probe, err := left.c.NamedProbe("is-propositional")
	astTestMust(t, err)
	goal, err := right.c.NewGoal(false, false, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	if _, err = probe.Apply(goal); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("foreign goal", err)
	}
	astTestMust(t, probe.Close())
	astTestMust(t, probe.Close())
	if _, err = probe.Apply(goal); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed probe", err)
	}
}
