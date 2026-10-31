package api

import "testing"

func TestAPIUnitSubsumptionTacticReuse(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	sort := f.pin(f.c.BoolSort())
	a := f.pin(f.c.Constant("unit-subsumption-a", sort))
	b := f.pin(f.c.Constant("unit-subsumption-b", sort))
	weak := f.pin(f.c.Or(a, b))
	tac, err := f.c.Tactic("unit-subsume-simplify")
	astTestMust(t, err)
	defer func() { astTestMust(t, tac.Close()) }()
	for attempt := 0; attempt < 2; attempt++ {
		g, err := f.c.NewGoal(false, false, false)
		astTestMust(t, err)
		astTestMust(t, g.Assert(weak))
		astTestMust(t, g.Assert(a))
		astTestMust(t, g.Assert(a))
		result, err := tac.Apply(g)
		astTestMust(t, err)
		n, err := result.NumSubgoals()
		astTestMust(t, err)
		if n != 1 {
			t.Fatal("subgoals", n)
		}
		subgoal, err := result.Subgoal(0)
		astTestMust(t, err)
		size, err := subgoal.Size()
		astTestMust(t, err)
		if size != 1 {
			t.Fatal("remaining formulas", size)
		}
		form, err := subgoal.Formula(0)
		astTestMust(t, err)
		value, err := f.m.RefGet(form)
		astTestMust(t, err)
		if value != a {
			t.Fatal("expected the last unit", value, a)
		}
		astTestMust(t, f.m.DelRef(&form))
		astTestMust(t, subgoal.Close())
		astTestMust(t, result.Close())
		astTestMust(t, g.Close())
	}
}
