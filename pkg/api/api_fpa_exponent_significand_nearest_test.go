package api

import "testing"

func TestFloatingPointExponentSignificandNearestTie(t *testing.T) {
	f := newFPAExponentSignificandFixture(t)
	value := f.convert(t, f.rounding, f.numeral(t, "0", f.integer), f.numeral(t, "16777217/16777216", f.real), f.single)
	result, found := f.rewrite(t, value)
	if !found {
		t.Fatal("nearest-even conversion was not reduced")
	}
	f.requireParts(t, result, 0, "0", "1")
}
