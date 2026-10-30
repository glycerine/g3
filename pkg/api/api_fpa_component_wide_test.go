package api

import "testing"

func TestFloatingPointComponentNumeralDouble(t *testing.T) {
	f := newFPANumeralPartsFixture(t)
	value, err := f.context.FloatingPointNumeralComponents(true, 1, 0x4000000000000, f.double)
	value = f.pin(t, value, err)
	if sign, err := f.context.FloatingPointNumeralSign(value); err != nil || sign != 1 {
		t.Fatal(sign, err)
	}
	if exponent, err := f.context.FloatingPointNumeralExponentString(value, false); err != nil || exponent != "1" {
		t.Fatal(exponent, err)
	}
	if significand, err := f.context.FloatingPointNumeralSignificandString(value); err != nil || significand != "1.25" {
		t.Fatal(significand, err)
	}
}
