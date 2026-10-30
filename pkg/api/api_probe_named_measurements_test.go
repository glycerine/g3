package api

import "testing"

func TestAPINamedProbeGoalMeasurements(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	goal, err := f.c.NewGoal(true, true, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	boolean := f.pin(f.m.MkBoolSort())
	name, err := f.m.Symbols.Intern("named-probe-p")
	astTestMust(t, err)
	p := f.pin(f.m.MkConst(name, boolean))
	astTestMust(t, goal.Assert(p))
	for name, want := range map[string]float64{"depth": 0, "size": 1, "num-exprs": 1, "produce-proofs": 0, "produce-model": 1, "produce-unsat-cores": 1} {
		probe, probeErr := f.c.NamedProbe(name)
		astTestMust(t, probeErr)
		value, applyErr := probe.Apply(goal)
		astTestMust(t, applyErr)
		if value != want {
			t.Fatalf("%s: %v, want %v", name, value, want)
		}
		astTestMust(t, probe.Close())
	}
}
