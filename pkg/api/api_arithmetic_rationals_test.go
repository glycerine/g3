package api

import "testing"

func TestSMTContextArithmeticRationalParts(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	r := f.pin(f.c.Real(-14, 6))
	num := f.pin(f.c.Numerator(r))
	den := f.pin(f.c.Denominator(r))
	if text, err := f.c.NumeralString(num); err != nil || text != "-7" {
		t.Fatal("numerator", text, err)
	}
	if text, err := f.c.NumeralString(den); err != nil || text != "3" {
		t.Fatal("denominator", text, err)
	}
}
