package api

import (
	"errors"
	"math"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestFloatingPointNumeralBitVectorWidthsAndValues(t *testing.T) {
	f := newFPANumeralPartsFixture(t)
	value, err := f.context.FloatingPointNumeralFloat32(-1.5, f.single)
	value = f.pin(t, value, err)
	sign, err := f.context.FloatingPointNumeralSignBit(value)
	sign = f.pin(t, sign, err)
	exponent, err := f.context.FloatingPointNumeralExponentBitVector(value, true)
	exponent = f.pin(t, exponent, err)
	unbiased, err := f.context.FloatingPointNumeralExponentBitVector(value, false)
	unbiased = f.pin(t, unbiased, err)
	significand, err := f.context.FloatingPointNumeralSignificandBitVector(value)
	significand = f.pin(t, significand, err)
	for _, row := range []struct {
		value ast.ASTHandle
		bits  uint32
		want  uint64
	}{{sign, 1, 1}, {exponent, 8, 127}, {unbiased, 8, 0}, {significand, 23, 0x400000}} {
		sort, queryErr := f.manager.GetSort(row.value)
		astTestMust(t, queryErr)
		bits, queryErr := f.manager.BitVectorSize(sort)
		if queryErr != nil || bits != row.bits || f.uint(t, row.value) != row.want {
			t.Fatal(bits, queryErr, f.uint(t, row.value), row.want)
		}
	}
}

func TestFloatingPointNumeralExponentBitVectorSpecialValues(t *testing.T) {
	f := newFPANumeralPartsFixture(t)
	for _, tc := range []struct {
		name             string
		makeValue        func() (ast.ASTHandle, error)
		biased, unbiased uint64
	}{
		{"positive infinity", func() (ast.ASTHandle, error) { return f.context.FloatingPointInfinity(f.single, false) }, 128, 128},
		{"negative infinity", func() (ast.ASTHandle, error) { return f.context.FloatingPointInfinity(f.single, true) }, 128, 128},
		{"negative zero", func() (ast.ASTHandle, error) { return f.context.FloatingPointZero(f.single, true) }, 0, 0},
		{"smallest subnormal", func() (ast.ASTHandle, error) {
			return f.context.FloatingPointNumeralFloat32(math.SmallestNonzeroFloat32, f.single)
		}, 0, 130},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value, err := tc.makeValue()
			value = f.pin(t, value, err)
			for _, row := range []struct {
				biased bool
				want   uint64
			}{{true, tc.biased}, {false, tc.unbiased}} {
				bits, e := f.context.FloatingPointNumeralExponentBitVector(value, row.biased)
				bits = f.pin(t, bits, e)
				if got := f.uint(t, bits); got != row.want {
					t.Fatalf("biased %v: got %d, want %d", row.biased, got, row.want)
				}
			}
		})
	}
}

func TestFloatingPointNumeralExponentNormalValues(t *testing.T) {
	f := newFPANumeralPartsFixture(t)
	for _, tc := range []struct {
		value            float32
		unbiased, biased int64
	}{{6, 2, 129}, {0.25, -2, 125}} {
		value, err := f.context.FloatingPointNumeralFloat32(tc.value, f.single)
		value = f.pin(t, value, err)
		for _, row := range []struct {
			biased bool
			want   int64
		}{{false, tc.unbiased}, {true, tc.biased}} {
			got, e := f.context.FloatingPointNumeralExponentInt64(value, row.biased)
			astTestMust(t, e)
			if got != row.want {
				t.Fatalf("%g biased %v: got %d, want %d", tc.value, row.biased, got, row.want)
			}
			bits, e := f.context.FloatingPointNumeralExponentBitVector(value, row.biased)
			bits = f.pin(t, bits, e)
			if encoded := f.uint(t, bits); encoded != uint64(uint8(row.want)) {
				t.Fatalf("%g biased %v: bitvector %d", tc.value, row.biased, encoded)
			}
		}
	}
}

func TestFloatingPointNumeralWideSignificandInspection(t *testing.T) {
	f := newFPANumeralPartsFixture(t)
	sort, err := f.context.FloatingPointQuadrupleSort()
	sort = f.pin(t, sort, err)
	value, err := f.context.FloatingPointNumeralFloat64(-1.5, sort)
	value = f.pin(t, value, err)
	sign, err := f.context.FloatingPointNumeralSignBit(value)
	sign = f.pin(t, sign, err)
	if got := f.uint(t, sign); got != 1 {
		t.Fatal("sign", got)
	}
	sig, err := f.context.FloatingPointNumeralSignificandBitVector(value)
	sig = f.pin(t, sig, err)
	text, err := f.context.NumeralString(sig)
	astTestMust(t, err)
	want := new(big.Int).Lsh(big.NewInt(1), 111).String()
	if text != want {
		t.Fatalf("significand %s, want %s", text, want)
	}
	if _, err = f.context.FloatingPointNumeralSignificandUint64(value); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("wide significand", err)
	}
}
