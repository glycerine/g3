package api

import "testing"

func TestAPIProbeAllSourceFactoriesConstructAndEvaluate(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	goal, err := f.c.NewGoal(false, false, false)
	astTestMust(t, err)
	defer func() { astTestMust(t, goal.Close()) }()
	names, err := f.c.ProbeNames()
	astTestMust(t, err)
	if len(names) != 42 {
		t.Fatal("source ADD_PROBE catalog size", len(names))
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			probe, err := f.c.NamedProbe(name)
			astTestMust(t, err)
			defer func() { astTestMust(t, probe.Close()) }()
			if _, err = probe.Apply(goal); err != nil {
				t.Fatal("registered source probe cannot evaluate an empty goal", err)
			}
		})
	}
}

func TestAPIProbeArithmeticFactoriesAndMixedComposition(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	goal, err := f.c.NewGoal(false, false, false)
	astTestMust(t, err)
	defer func() { astTestMust(t, goal.Close()) }()
	real := f.pin(f.c.RealSort())
	x := f.pin(f.c.Constant("probe-nonlinear-real", real))
	two := f.pin(f.c.Numeral("2", real))
	squared := f.pin(f.c.ArithmeticMul(x, x))
	formula := f.pin(f.c.Equal(squared, two))
	astTestMust(t, goal.Assert(formula))
	for name, want := range map[string]float64{"arith-max-deg": 2, "arith-max-bw": 2, "is-qfnra": 1, "is-qflra": 0, "is-pb": 0} {
		probe, err := f.c.NamedProbe(name)
		astTestMust(t, err)
		actual, err := probe.Apply(goal)
		astTestMust(t, err)
		astTestMust(t, probe.Close())
		if actual != want {
			t.Fatal("source arithmetic probe", name, actual, want)
		}
	}
	degree, err := f.c.NamedProbe("arith-max-deg")
	astTestMust(t, err)
	constant, err := f.c.ConstantProbe(2)
	astTestMust(t, err)
	comparison, err := degree.Compare(APIProbeEqual, constant)
	astTestMust(t, err)
	astTestMust(t, degree.Close())
	astTestMust(t, constant.Close())
	actual, err := comparison.Apply(goal)
	astTestMust(t, err)
	astTestMust(t, comparison.Close())
	if actual != 1 {
		t.Fatal("mixed concrete probe composition", actual)
	}
}
