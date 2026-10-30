package arith

import "testing"

func fmSubsumptionConstraint(lits []int32, vars []uint32, coeffs []int64, bound int64, strict bool) *FMConstraint {
	c := &FMConstraint{Literals: lits, Variables: vars, Constant: fmCRat(bound, 1), Strict: strict}
	for _, v := range coeffs {
		c.Coefficients = append(c.Coefficients, fmCRat(v, 1))
	}
	return c
}

func TestFMConstraintSubsumesPermutationAndLiterals(t *testing.T) {
	a := fmSubsumptionConstraint([]int32{2}, []uint32{3, 1}, []int64{4, -2}, 7, false)
	b := fmSubsumptionConstraint([]int32{-5, 2}, []uint32{1, 3}, []int64{-2, 4}, 9, false)
	if !FMConstraintSubsumes(a, b) {
		t.Fatal("stronger permuted constraint did not subsume")
	}
	b.Literals[1] = -2
	if FMConstraintSubsumes(a, b) {
		t.Fatal("opposite literal was accepted")
	}
}

func TestFMConstraintSubsumesStrictBoundary(t *testing.T) {
	nonstrict := fmSubsumptionConstraint(nil, []uint32{0}, []int64{1}, 4, false)
	strict := fmSubsumptionConstraint(nil, []uint32{0}, []int64{1}, 4, true)
	if FMConstraintSubsumes(nonstrict, strict) || !FMConstraintSubsumes(strict, nonstrict) {
		t.Fatal("strict tie ordering is reversed")
	}
}
