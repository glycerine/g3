package api

import "testing"

func TestFloatingPointNumeralStrings(t *testing.T) {
	f := newFPANumeralPartsFixture(t)
	value, err := f.context.FloatingPointNumeralFloat32(1.5, f.single)
	value = f.pin(t, value, err)
	if exponent, err := f.context.FloatingPointNumeralExponentString(value, false); err != nil || exponent != "0" {
		t.Fatal(exponent, err)
	}
	if exponent, err := f.context.FloatingPointNumeralExponentString(value, true); err != nil || exponent != "127" {
		t.Fatal(exponent, err)
	}
	if significand, err := f.context.FloatingPointNumeralSignificandString(value); err != nil || significand != "1.5" {
		t.Fatal(significand, err)
	}
}
