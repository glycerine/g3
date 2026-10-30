package api

import "testing"

func TestFloatingPointNativeFloat64NumeralParts(t *testing.T) {
	f := newFPANumeralPartsFixture(t)
	value, err := f.context.FloatingPointNumeralFloat64(-2.5, f.double)
	value = f.pin(t, value, err)
	if sign, err := f.context.FloatingPointNumeralSign(value); err != nil || sign != 1 {
		t.Fatal(sign, err)
	}
	if exponent, err := f.context.FloatingPointNumeralExponentInt64(value, true); err != nil || exponent != 1024 {
		t.Fatal(exponent, err)
	}
	if exponent, err := f.context.FloatingPointNumeralExponentInt64(value, false); err != nil || exponent != 1 {
		t.Fatal(exponent, err)
	}
	if significand, err := f.context.FloatingPointNumeralSignificandUint64(value); err != nil || significand != 0x4000000000000 {
		t.Fatalf("significand %#x %v", significand, err)
	}
}
