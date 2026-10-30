package api

import "testing"

func TestFloatingPointExponentSignificandDirectedTie(t *testing.T) {
	f := newFPAExponentSignificandFixture(t)
	rounding, err := f.context.FloatingPointRoundTowardPositiveValue()
	rounding = f.pin(t, rounding, err)
	value := f.convert(t, rounding, f.numeral(t, "0", f.integer), f.numeral(t, "16777217/16777216", f.real), f.single)
	result, found := f.rewrite(t, value)
	if !found {
		t.Fatal("toward-positive conversion was not reduced")
	}
	f.requireParts(t, result, 0, "0", "1.00000011920928955078125")
}
