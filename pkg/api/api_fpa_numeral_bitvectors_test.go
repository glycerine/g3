package api

import (
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
