package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextFloatingPointConversions(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	single := f.pin(f.c.FloatingPointSingleSort())
	half := f.pin(f.c.FloatingPointHalfSort())
	rounding := f.pin(f.c.FloatingPointRoundNearestEven())
	zero := f.pin(f.c.FloatingPointZero(half, false))
	ieeeSort := f.pin(f.c.BitVectorSort(32))
	ieee := f.pin(f.c.BitVectorValue(0, ieeeSort))
	integerSort := f.pin(f.c.BitVectorSort(12))
	integer := f.pin(f.c.BitVectorValue(-3, integerSort))
	real := f.pin(f.c.Real(3, 2))

	tests := []struct {
		kind int32
		sort ast.ASTHandle
		make func() (ast.ASTHandle, error)
	}{
		{ast.FPAOpToFP, single, func() (ast.ASTHandle, error) { return f.c.FloatingPointFromIEEEBitVector(ieee, single) }},
		{ast.FPAOpToFP, single, func() (ast.ASTHandle, error) { return f.c.FloatingPointConvert(rounding, zero, single) }},
		{ast.FPAOpToFP, single, func() (ast.ASTHandle, error) { return f.c.FloatingPointFromReal(rounding, real, single) }},
		{ast.FPAOpToFP, single, func() (ast.ASTHandle, error) { return f.c.FloatingPointFromSignedBitVector(rounding, integer, single) }},
		{ast.FPAOpToFPUnsigned, single, func() (ast.ASTHandle, error) {
			return f.c.FloatingPointFromUnsignedBitVector(rounding, integer, single)
		}},
	}
	for _, test := range tests {
		value := f.pin(test.make())
		if kind, err := f.c.DeclarationKind(value); err != nil || kind != test.kind {
			t.Fatal("conversion kind", test.kind, kind, err)
		}
		if sort := f.pin(f.c.SortOf(value)); sort != test.sort {
			t.Fatal("conversion sort", test.kind, sort, test.sort)
		}
	}

	toUnsigned := f.pin(f.c.FloatingPointToUnsignedBitVector(rounding, zero, 9))
	toSigned := f.pin(f.c.FloatingPointToSignedBitVector(rounding, zero, 10))
	for _, test := range []struct {
		value ast.ASTHandle
		kind  int32
		bits  uint32
	}{{toUnsigned, ast.FPAOpToUBV, 9}, {toSigned, ast.FPAOpToSBV, 10}} {
		if kind, err := f.c.DeclarationKind(test.value); err != nil || kind != test.kind {
			t.Fatal("bit-vector conversion kind", kind, err)
		}
		sort := f.pin(f.c.SortOf(test.value))
		if bits, err := f.c.BitVectorSortSize(sort); err != nil || bits != test.bits {
			t.Fatal("bit-vector conversion width", bits, err)
		}
	}
	toReal := f.pin(f.c.FloatingPointToReal(zero))
	if kind, err := f.c.DeclarationKind(toReal); err != nil || kind != ast.FPAOpToReal {
		t.Fatal("to-real", kind, err)
	}
	toIEEE := f.pin(f.c.FloatingPointToIEEEBitVector(zero))
	if kind, err := f.c.DeclarationKind(toIEEE); err != nil || kind != ast.FPAOpToIEEEBV {
		t.Fatal("to-ieee", kind, err)
	}
	if _, err := f.c.FloatingPointToUnsignedBitVector(rounding, zero, 0); err == nil {
		t.Fatal("zero-width conversion accepted")
	}
}
