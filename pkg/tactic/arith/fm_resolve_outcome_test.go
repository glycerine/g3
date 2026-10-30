package arith

import "testing"

func fmResolutionFixture(t *testing.T) (*FMConstraintStore, *FMVariableTable) {
	t.Helper()
	_, m := fixDLFixture(t, false)
	store, e := NewFMConstraintStore(m)
	equationMust(t, e)
	t.Cleanup(func() { equationMust(t, store.Close()) })
	return store, NewFMVariableTable()
}

func TestFMResolveTautologyAndInconsistency(t *testing.T) {
	store, v := fmResolutionFixture(t)
	x := v.Add(false, false)
	tautLower := fmSubsumptionConstraint(nil, []uint32{x}, []int64{-1}, 2, false)
	tautUpper := fmSubsumptionConstraint(nil, []uint32{x}, []int64{1}, 3, false)
	r, e := FMResolve(store, v, tautLower, tautUpper, x)
	equationMust(t, e)
	if !r.Tautology || r.Constraint != nil {
		t.Fatal("true consequent was materialized")
	}
	badLower := fmSubsumptionConstraint(nil, []uint32{x}, []int64{-1}, -1, false)
	badUpper := fmSubsumptionConstraint(nil, []uint32{x}, []int64{1}, 0, false)
	r, e = FMResolve(store, v, badLower, badUpper, x)
	equationMust(t, e)
	if !r.Inconsistent || r.Tautology || r.Constraint != nil {
		t.Fatal("false consequent was not reported inconsistent")
	}
}

func TestFMResolveClauseTautologyPrecedesAllocation(t *testing.T) {
	store, v := fmResolutionFixture(t)
	x := v.Add(false, false)
	lower := fmSubsumptionConstraint([]int32{3}, []uint32{x}, []int64{-1}, -1, false)
	upper := fmSubsumptionConstraint([]int32{-3}, []uint32{x}, []int64{1}, 0, false)
	before := len(store.constraints)
	r, e := FMResolve(store, v, lower, upper, x)
	equationMust(t, e)
	if !r.Tautology || len(store.constraints) != before {
		t.Fatal("clause tautology allocated a constraint")
	}
}
