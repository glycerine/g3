package arith

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"testing"
)

func TestFMConstraintMakeIntegerAndNormalize(t *testing.T) {
	c := &FMConstraint{numerals: fmTestNumerals, Variables: []uint32{0, 1}, Coefficients: []unumeral.Rational{fmCRat(1, 2), fmCRat(-3, 4)}, Constant: fmCRat(5, 6)}
	c.MakeInteger()
	if fmText(&c.Coefficients[0]) != "6" || fmText(&c.Coefficients[1]) != "-9" || fmText(&c.Constant) != "10" {
		t.Fatalf("integer normalization = %s,%s <= %s", fmText(&c.Coefficients[0]), fmText(&c.Coefficients[1]), fmText(&c.Constant))
	}
	d := &FMConstraint{numerals: fmTestNumerals, Variables: []uint32{0, 1}, Coefficients: []unumeral.Rational{fmCRat(6, 1), fmCRat(-9, 1)}, Constant: fmCRat(12, 1)}
	d.NormalizeCoefficients()
	if fmText(&d.Coefficients[0]) != "2" || fmText(&d.Coefficients[1]) != "-3" || fmText(&d.Constant) != "4" {
		t.Fatalf("gcd normalization = %s,%s <= %s", fmText(&d.Coefficients[0]), fmText(&d.Coefficients[1]), fmText(&d.Constant))
	}
}

func TestFMConstraintNormalizeLeavesNonintegralInput(t *testing.T) {
	c := &FMConstraint{numerals: fmTestNumerals, Variables: []uint32{0}, Coefficients: []unumeral.Rational{fmCRat(3, 2)}, Constant: fmCRat(6, 1)}
	c.NormalizeCoefficients()
	if fmText(&c.Coefficients[0]) != "3/2" || fmText(&c.Constant) != "6" {
		t.Fatal("nonintegral constraint was changed")
	}
}
