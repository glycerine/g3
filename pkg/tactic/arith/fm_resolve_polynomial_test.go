package arith

import "testing"

func TestFMResolvePolynomialCombinesAndCancels(t *testing.T) {
	v := NewFMVariableTable()
	x, y := v.Add(false, false), v.Add(false, false)
	lower := fmSubsumptionConstraint(nil, []uint32{x, y}, []int64{-2, 3}, 5, false)
	upper := fmSubsumptionConstraint(nil, []uint32{x, y}, []int64{4, 1}, 7, false)
	xs, as, c, strict, e := fmResolvePolynomial(v, lower, upper, x)
	equationMust(t, e)
	if strict || len(xs) != 1 || xs[0] != y || as[0].RatString() != "14" || c.RatString() != "34" {
		t.Fatalf("resolution polynomial = %v %v <= %s strict=%v", xs, as, c.RatString(), strict)
	}
	upper.Coefficients[1] = fmCRat(-6, 1)
	xs, as, _, _, e = fmResolvePolynomial(v, lower, upper, x)
	equationMust(t, e)
	if len(xs) != 0 || len(as) != 0 {
		t.Fatal("zero coefficient was retained")
	}
}

func TestFMResolvePolynomialRoundsIntegerStrictResult(t *testing.T) {
	v := NewFMVariableTable()
	x, y := v.Add(true, false), v.Add(true, false)
	lower := fmSubsumptionConstraint(nil, []uint32{x, y}, []int64{-1, 1}, 2, true)
	upper := fmSubsumptionConstraint(nil, []uint32{x}, []int64{1}, 3, false)
	_, as, c, strict, e := fmResolvePolynomial(v, lower, upper, x)
	equationMust(t, e)
	if strict || len(as) != 1 || as[0].RatString() != "1" || c.RatString() != "4" {
		t.Fatalf("integer result = %v <= %s strict=%v", as, c.RatString(), strict)
	}
}
