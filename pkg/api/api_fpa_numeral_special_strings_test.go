package api

import (
	"math"
	"testing"
)

func TestFloatingPointSpecialNumeralStrings(t *testing.T) {
	f := newFPANumeralPartsFixture(t)
	zero, err := f.context.FloatingPointZero(f.single, false)
	zero = f.pin(t, zero, err)
	if exponent, err := f.context.FloatingPointNumeralExponentString(zero, true); err != nil || exponent != "0" {
		t.Fatal(exponent, err)
	}
	if significand, err := f.context.FloatingPointNumeralSignificandString(zero); err != nil || significand != "1" {
		t.Fatal(significand, err)
	}
	infinity, err := f.context.FloatingPointInfinity(f.single, false)
	infinity = f.pin(t, infinity, err)
	if exponent, err := f.context.FloatingPointNumeralExponentString(infinity, false); err != nil || exponent != "128" {
		t.Fatal(exponent, err)
	}
	if significand, err := f.context.FloatingPointNumeralSignificandString(infinity); err != nil || significand != "0" {
		t.Fatal(significand, err)
	}
	subnormal, err := f.context.FloatingPointNumeralFloat32(math.SmallestNonzeroFloat32, f.single)
	subnormal = f.pin(t, subnormal, err)
	if exponent, err := f.context.FloatingPointNumeralExponentString(subnormal, false); err != nil || exponent != "-126" {
		t.Fatal(exponent, err)
	}
	if significand, err := f.context.FloatingPointNumeralSignificandString(subnormal); err != nil || significand != "0.00000011920928955078125" {
		t.Fatal(significand, err)
	}
}
