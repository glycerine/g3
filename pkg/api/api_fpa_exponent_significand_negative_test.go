package api

import "testing"

func TestFloatingPointExponentSignificandNegative(t *testing.T) {
	f := newFPAExponentSignificandFixture(t)
	value := f.convert(t, f.rounding, f.numeral(t, "-1", f.integer), f.numeral(t, "-3/2", f.real), f.single)
	result, found := f.rewrite(t, value)
	if !found {
		t.Fatal("negative concrete conversion was not reduced")
	}
	f.requireParts(t, result, 1, "-1", "1.5")
}
