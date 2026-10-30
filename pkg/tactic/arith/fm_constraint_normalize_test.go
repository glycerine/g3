package arith

import (
	"math/big"
	"testing"
)

func fmCRat(n, d int64) big.Rat { return *big.NewRat(n, d) }

func TestFMConstraintMakeIntegerAndNormalize(t *testing.T) {
	c := &FMConstraint{Variables: []uint32{0, 1}, Coefficients: []big.Rat{fmCRat(1, 2), fmCRat(-3, 4)}, Constant: fmCRat(5, 6)}
	c.MakeInteger()
	if c.Coefficients[0].RatString() != "6" || c.Coefficients[1].RatString() != "-9" || c.Constant.RatString() != "10" {
		t.Fatalf("integer normalization = %s,%s <= %s", c.Coefficients[0].RatString(), c.Coefficients[1].RatString(), c.Constant.RatString())
	}
	d := &FMConstraint{Variables: []uint32{0, 1}, Coefficients: []big.Rat{fmCRat(6, 1), fmCRat(-9, 1)}, Constant: fmCRat(12, 1)}
	d.NormalizeCoefficients()
	if d.Coefficients[0].RatString() != "2" || d.Coefficients[1].RatString() != "-3" || d.Constant.RatString() != "4" {
		t.Fatalf("gcd normalization = %s,%s <= %s", d.Coefficients[0].RatString(), d.Coefficients[1].RatString(), d.Constant.RatString())
	}
}

func TestFMConstraintNormalizeLeavesNonintegralInput(t *testing.T) {
	c := &FMConstraint{Variables: []uint32{0}, Coefficients: []big.Rat{fmCRat(3, 2)}, Constant: fmCRat(6, 1)}
	c.NormalizeCoefficients()
	if c.Coefficients[0].RatString() != "3/2" || c.Constant.RatString() != "6" {
		t.Fatal("nonintegral constraint was changed")
	}
}
