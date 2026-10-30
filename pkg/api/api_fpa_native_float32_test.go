package api

import "testing"

func TestFloatingPointNativeFloat32NumeralParts(t *testing.T) {
	f := newFPANumeralPartsFixture(t)
	value, err := f.context.FloatingPointNumeralFloat32(1.5, f.single)
	value = f.pin(t, value, err)
	if sign, err := f.context.FloatingPointNumeralSign(value); err != nil || sign != 0 {
		t.Fatal(sign, err)
	}
	if exponent, err := f.context.FloatingPointNumeralExponentInt64(value, true); err != nil || exponent != 127 {
		t.Fatal(exponent, err)
	}
	if exponent, err := f.context.FloatingPointNumeralExponentInt64(value, false); err != nil || exponent != 0 {
		t.Fatal(exponent, err)
	}
	if significand, err := f.context.FloatingPointNumeralSignificandUint64(value); err != nil || significand != 0x400000 {
		t.Fatalf("significand %#x %v", significand, err)
	}
}
