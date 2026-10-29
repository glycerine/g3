package qe

import "testing"

func TestArithmeticModelProjectIntegerSelectsSatisfiedUpperBound(t *testing.T) {
	f := newArithmeticModelFixture(t, false, "5")
	m := f.r.manager
	zero, minusOne := f.number("0"), f.number("-1")
	negativeX := f.own(m.MkArithmeticMul(minusOne, f.x))
	lower0 := f.own(m.MkArithmeticLE(negativeX, zero))
	lower1 := f.own(m.MkArithmeticLE(f.own(m.MkArithmeticAdd(negativeX, minusOne)), zero))
	upper := f.own(m.MkArithmeticLE(f.own(m.MkArithmeticAdd(f.x, f.number("-5"))), zero))
	original, _ := f.project(t, lower0, lower1, upper)
	if f.plugin.branches[ArithmeticBranchKey{Formula: original, Variable: f.x, Branch: 1}] == nil {
		t.Fatal("integer projection did not select source upper-bound branch")
	}
}
