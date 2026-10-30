package api

import "testing"

func TestAPIOptimizationObjectivesFollowScopes(t *testing.T) {
	f := newSMTAPIFixture(t, "QF_LIA", nil)
	x := f.arithmeticVariable("objective-scope-x", true)
	astTestMust(t, f.c.Minimize(x))
	astTestMust(t, f.c.Push())
	astTestMust(t, f.c.Maximize(x))
	if count, err := f.c.OptimizationObjectiveCount(); err != nil || count != 2 {
		t.Fatal("pushed count", count, err)
	}
	astTestMust(t, f.c.Pop(1))
	if count, err := f.c.OptimizationObjectiveCount(); err != nil || count != 1 {
		t.Fatal("popped count", count, err)
	}
	objectives, err := f.c.OptimizationObjectives()
	astTestMust(t, err)
	handle, err := f.m.RefGet(objectives[0])
	astTestMust(t, err)
	if handle != x {
		t.Fatal("remaining objective", handle, x)
	}
	astTestMust(t, f.m.DelRef(&objectives[0]))
}
