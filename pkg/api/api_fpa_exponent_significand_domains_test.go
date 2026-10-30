package api

import "testing"

func TestFloatingPointExponentSignificandDomainGuards(t *testing.T) {
	f := newFPAExponentSignificandFixture(t)
	integer := f.numeral(t, "2", f.integer)
	real := f.numeral(t, "3/2", f.real)
	if _, err := f.context.FloatingPointFromExponentSignificand(integer, integer, real, f.single); err == nil {
		t.Fatal("integer rounding mode accepted")
	}
	if _, err := f.context.FloatingPointFromExponentSignificand(f.rounding, real, real, f.single); err == nil {
		t.Fatal("Real exponent accepted")
	}
	if _, err := f.context.FloatingPointFromExponentSignificand(f.rounding, integer, integer, f.single); err == nil {
		t.Fatal("Int significand accepted")
	}
}
