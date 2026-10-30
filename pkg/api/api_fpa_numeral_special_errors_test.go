package api

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFloatingPointNumeralSpecialValuesAndErrors(t *testing.T) {
	f := newFPANumeralPartsFixture(t)
	zero, err := f.context.FloatingPointZero(f.single, true)
	zero = f.pin(t, zero, err)
	if sign, err := f.context.FloatingPointNumeralSign(zero); err != nil || sign != 1 {
		t.Fatal(sign, err)
	}
	if exponent, err := f.context.FloatingPointNumeralExponentInt64(zero, false); err != nil || exponent != 0 {
		t.Fatal(exponent, err)
	}
	infinity, err := f.context.FloatingPointInfinity(f.single, false)
	infinity = f.pin(t, infinity, err)
	if exponent, err := f.context.FloatingPointNumeralExponentInt64(infinity, false); err != nil || exponent != 128 {
		t.Fatal(exponent, err)
	}
	if significand, err := f.context.FloatingPointNumeralSignificandUint64(infinity); err != nil || significand != 0 {
		t.Fatal(significand, err)
	}
	subnormal, err := f.context.FloatingPointNumeralFloat32(math.SmallestNonzeroFloat32, f.single)
	subnormal = f.pin(t, subnormal, err)
	if exponent, err := f.context.FloatingPointNumeralExponentInt64(subnormal, false); err != nil || exponent != -126 {
		t.Fatal(exponent, err)
	}
	if exponent, err := f.context.FloatingPointNumeralExponentInt64(subnormal, true); err != nil || exponent != 0 {
		t.Fatal(exponent, err)
	}
	nan, err := f.context.FloatingPointNaN(f.single)
	nan = f.pin(t, nan, err)
	if _, err = f.context.FloatingPointNumeralSign(nan); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("NaN sign accepted", err)
	}
	truth, err := f.manager.MkTrue()
	truth = f.pin(t, truth, err)
	if _, err = f.context.FloatingPointNumeralExponentInt64(truth, true); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("non-FPA accepted", err)
	}
	if _, err = f.context.FloatingPointNumeralFloat64(1, truth); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("non-sort accepted", err)
	}
}
