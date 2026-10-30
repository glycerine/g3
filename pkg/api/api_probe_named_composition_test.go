package api

import "testing"

func TestAPINamedProbeComposesWithPublicProbeAPI(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	goal, err := f.c.NewGoal(false, false, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	size, err := f.c.NamedProbe("size")
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, size.Close()) })
	zero, err := f.c.ConstantProbe(0)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, zero.Close()) })
	equal, err := size.Compare(APIProbeEqual, zero)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, equal.Close()) })
	value, err := equal.Apply(goal)
	astTestMust(t, err)
	if value != 1 {
		t.Fatal("named/composed probe", value)
	}
}
