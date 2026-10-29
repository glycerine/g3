package api

import "testing"

func TestAPINumeralMachineExtractors(t *testing.T) {
	f := newAPIAlgebraicFixture(t)
	negative := f.pin(f.c.Numeral("-17", f.pin(f.c.IntegerSort())))
	if value, matched, err := f.c.NumeralInt32(negative); err != nil || !matched || value != -17 {
		t.Fatal("int32", value, matched, err)
	}
	if value, matched, err := f.c.NumeralUint32(f.three); err != nil || !matched || value != 3 {
		t.Fatal("uint32", value, matched, err)
	}
	if numerator, denominator, matched, err := f.c.NumeralSmall(f.half); err != nil || !matched || numerator != 1 || denominator != 2 {
		t.Fatal("small rational", numerator, denominator, matched, err)
	}
}
