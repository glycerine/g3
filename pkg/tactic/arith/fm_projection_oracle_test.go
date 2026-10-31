package arith

import (
	"math/big"
	"testing"
)

// The oracle asks whether the two source bounds have a nonempty interval for
// real x. It does not construct a resolvent or use the eliminator's formula.
func TestFMProjectionMatchesIndependentIntervalOracle(t *testing.T) {
	checked := 0
	for _, integerY := range []bool{false, true} {
		vars := NewFMVariableTable()
		x, y := vars.Add(false, false), vars.Add(integerY, false)
		for a := int64(1); a <= 2; a++ {
			for b := int64(1); b <= 2; b++ {
				for p := int64(-1); p <= 1; p++ {
					for q := int64(-1); q <= 1; q++ {
						for c := int64(-1); c <= 1; c++ {
							for d := int64(-1); d <= 1; d++ {
								for _, lowerStrict := range []bool{false, true} {
									for _, upperStrict := range []bool{false, true} {
										lower := fmSubsumptionConstraint(nil, []uint32{x, y}, []int64{-a, p}, c, lowerStrict)
										upper := fmSubsumptionConstraint(nil, []uint32{x, y}, []int64{b, q}, d, upperStrict)
										xs, coeffs, bound, strict, err := fmResolvePolynomial(vars, lower, upper, x)
										equationMust(t, err)
										for value := int64(-2); value <= 2; value++ {
											lowerEnd := new(big.Rat).Quo(big.NewRat(c-p*value, 1), big.NewRat(-a, 1))
											upperEnd := new(big.Rat).Quo(big.NewRat(d-q*value, 1), big.NewRat(b, 1))
											comparison := lowerEnd.Cmp(upperEnd)
											want := comparison < 0 || comparison == 0 && !lowerStrict && !upperStrict
											lhs := new(big.Rat)
											for i, v := range xs {
												if v != y {
													t.Fatal("eliminated variable survived", v)
												}
												lhs.Add(lhs, new(big.Rat).Mul(&coeffs[i], big.NewRat(value, 1)))
											}
											comparison = lhs.Cmp(&bound)
											got := comparison < 0 || comparison == 0 && !strict
											checked++
											if got != want {
												t.Fatalf("integerY=%v a=%d b=%d p=%d q=%d c=%d d=%d strict=%v/%v y=%d: interval=%s..%s projection=%s <= %s strict=%v", integerY, a, b, p, q, c, d, lowerStrict, upperStrict, value, lowerEnd, upperEnd, lhs, &bound, strict)
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
	if checked != 12960 {
		t.Fatal("oracle coverage", checked)
	}
}
