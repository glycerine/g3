package api

import "testing"

func TestFloatingPointExponentSignificandTargetGuard(t *testing.T) {
	f := newFPAExponentSignificandFixture(t)
	integer := f.numeral(t, "2", f.integer)
	real := f.numeral(t, "3/2", f.real)
	if _, err := f.context.FloatingPointFromExponentSignificand(f.rounding, integer, real, f.real); err == nil {
		t.Fatal("non-floating-point target accepted")
	}
}
