package arith

import "testing"

func TestFMCanEliminateIntegerUnitSide(t *testing.T) {
	v := NewFMVariableTable()
	x, y := v.Add(true, false), v.Add(true, false)
	lower := fmSubsumptionConstraint(nil, []uint32{x, y}, []int64{-1, 3}, 2, false)
	upper := fmSubsumptionConstraint(nil, []uint32{x}, []int64{4}, 9, false)
	v.Index.Lower[x], v.Index.Upper[x] = []*FMConstraint{lower}, []*FMConstraint{upper}
	if !v.CanEliminate(x) {
		t.Fatal("unit lower side did not permit integer elimination")
	}
	lower.Coefficients[0] = fmCRat(-2, 1)
	if v.CanEliminate(x) {
		t.Fatal("two non-unit sides permitted integer elimination")
	}
}

func TestFMCanEliminateRejectsMixedIntegerBounds(t *testing.T) {
	v := NewFMVariableTable()
	x, real := v.Add(true, false), v.Add(false, false)
	v.Index.Lower[x] = []*FMConstraint{fmSubsumptionConstraint(nil, []uint32{x, real}, []int64{-1, 1}, 0, false)}
	if v.CanEliminate(x) {
		t.Fatal("mixed real bound permitted integer elimination")
	}
	if !v.CanEliminate(real) {
		t.Fatal("real variable should always be eliminable")
	}
}
