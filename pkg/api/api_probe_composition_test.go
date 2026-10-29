package api

import (
	"errors"
	"testing"
)

func TestAPIProbeComparisonAndBooleanComposition(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	goal, err := f.c.NewGoal(false, false, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	one, err := f.c.ConstantProbe(1)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, one.Close()) })
	two, err := f.c.ConstantProbe(2)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, two.Close()) })
	less, err := one.Compare(APIProbeLess, two)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, less.Close()) })
	greater, err := one.Compare(APIProbeGreater, two)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, greater.Close()) })
	notGreater, err := greater.Not()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, notGreater.Close()) })
	both, err := less.And(notGreater)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, both.Close()) })
	either, err := greater.Or(less)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, either.Close()) })
	for name, probe := range map[string]*APIProbe{"less": less, "not-greater": notGreater, "both": both, "either": either} {
		value, applyErr := probe.Apply(goal)
		if applyErr != nil || value != 1 {
			t.Fatalf("%s: %v, %v", name, value, applyErr)
		}
	}
	if _, err = one.Compare(APIProbeComparison(255), two); err == nil {
		t.Fatal("invalid comparison accepted")
	}
}

func TestAPIProbeRejectsForeignOwners(t *testing.T) {
	left := newSMTAPIFixture(t, "ALL", nil)
	right := newSMTAPIFixture(t, "ALL", nil)
	a, err := left.c.ConstantProbe(1)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, a.Close()) })
	b, err := right.c.ConstantProbe(2)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, b.Close()) })
	if _, err = a.And(b); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("foreign probe", err)
	}
	goal, err := right.c.NewGoal(false, false, false)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, goal.Close()) })
	if _, err = a.Apply(goal); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("foreign goal", err)
	}
}
