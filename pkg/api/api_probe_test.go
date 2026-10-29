package api

import (
	"errors"
	"testing"
)

func TestAPIProbeGoalMeasurementsAndLifecycle(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	goal, err := f.c.NewGoal(true, true, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	boolean := f.pin(f.c.BoolSort())
	x := f.pin(f.c.Constant("probe-x", boolean))
	astTestMust(t, goal.Assert(x))

	cases := []struct {
		kind APIGoalProbeKind
		want float64
	}{
		{APIGoalProbeDepth, 0},
		{APIGoalProbeSize, 1},
		{APIGoalProbeProduceProofs, 0},
		{APIGoalProbeProduceModels, 1},
		{APIGoalProbeProduceUnsatCores, 1},
	}
	for _, test := range cases {
		probe, probeErr := f.c.GoalProbe(test.kind)
		astTestMust(t, probeErr)
		got, applyErr := probe.Apply(goal)
		astTestMust(t, applyErr)
		if got != test.want {
			t.Fatalf("probe %d: got %v want %v", test.kind, got, test.want)
		}
		astTestMust(t, probe.Close())
		if _, applyErr = probe.Apply(goal); !errors.Is(applyErr, ErrSMTContextOwner) {
			t.Fatalf("closed probe %d: %v", test.kind, applyErr)
		}
		astTestMust(t, probe.Close())
	}
	if _, err = f.c.GoalProbe(APIGoalProbeKind(255)); err == nil {
		t.Fatal("invalid goal probe accepted")
	}
}
