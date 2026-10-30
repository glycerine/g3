package api

import "testing"

func TestFloatingPointExponentSignificandPositive(t *testing.T) {
	f := newFPAExponentSignificandFixture(t)
	value := f.convert(t, f.rounding, f.numeral(t, "2", f.integer), f.numeral(t, "3/2", f.real), f.single)
	result, found := f.rewrite(t, value)
	if !found {
		t.Fatal("concrete conversion was not reduced")
	}
	f.requireParts(t, result, 0, "2", "1.5")
}
