package api

import "testing"

func TestAPINamedProbeConstantClasses(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	goal, err := f.c.NewGoal(false, false, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	boolean := f.pin(f.m.MkBoolSort())
	boolName, err := f.m.Symbols.Intern("named-probe-b")
	astTestMust(t, err)
	b := f.pin(f.m.MkConst(boolName, boolean))
	x := f.arithmeticVariable("named-probe-x", true)
	equality := f.pin(f.m.MkEq(x, x))
	conjunction := f.pin(f.m.MkAnd(b, equality))
	astTestMust(t, goal.Assert(conjunction))
	for name, want := range map[string]float64{"num-consts": 1, "num-bool-consts": 1, "num-arith-consts": 1, "num-bv-consts": 0} {
		probe, probeErr := f.c.NamedProbe(name)
		astTestMust(t, probeErr)
		value, applyErr := probe.Apply(goal)
		astTestMust(t, applyErr)
		astTestMust(t, probe.Close())
		if value != want {
			t.Fatalf("%s: %v, want %v", name, value, want)
		}
	}
}
