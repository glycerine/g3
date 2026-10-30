package api

import "testing"

func TestFloatingPointNativeInt32NumeralParts(t *testing.T) {
	f := newFPANumeralPartsFixture(t)
	value, err := f.context.FloatingPointNumeralInt32(-3, f.single)
	value = f.pin(t, value, err)
	if sign, err := f.context.FloatingPointNumeralSign(value); err != nil || sign != 1 {
		t.Fatal(sign, err)
	}
	if exponent, err := f.context.FloatingPointNumeralExponentInt64(value, true); err != nil || exponent != 128 {
		t.Fatal(exponent, err)
	}
	if significand, err := f.context.FloatingPointNumeralSignificandUint64(value); err != nil || significand != 0x400000 {
		t.Fatalf("significand %#x %v", significand, err)
	}
}
