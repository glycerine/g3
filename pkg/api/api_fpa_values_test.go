package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextFloatingPointSpecialValuesAndComponents(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	sort := f.pin(f.c.FloatingPointSingleSort())
	tests := []struct {
		make func() (ast.ASTHandle, error)
		kind int32
	}{
		{func() (ast.ASTHandle, error) { return f.c.FloatingPointNaN(sort) }, ast.FPAOpNaN},
		{func() (ast.ASTHandle, error) { return f.c.FloatingPointInfinity(sort, false) }, ast.FPAOpPositiveInf},
		{func() (ast.ASTHandle, error) { return f.c.FloatingPointInfinity(sort, true) }, ast.FPAOpNegativeInf},
		{func() (ast.ASTHandle, error) { return f.c.FloatingPointZero(sort, false) }, ast.FPAOpPositiveZero},
		{func() (ast.ASTHandle, error) { return f.c.FloatingPointZero(sort, true) }, ast.FPAOpNegativeZero},
	}
	for _, test := range tests {
		value := f.pin(test.make())
		if kind, err := f.c.DeclarationKind(value); err != nil || kind != test.kind {
			t.Fatal("special value", test.kind, kind, err)
		}
		if got := f.pin(f.c.SortOf(value)); got != sort {
			t.Fatal("special value sort", got, sort)
		}
	}

	signSort := f.pin(f.c.BitVectorSort(1))
	exponentSort := f.pin(f.c.BitVectorSort(8))
	significandSort := f.pin(f.c.BitVectorSort(23))
	sign := f.pin(f.c.BitVectorValue(0, signSort))
	exponent := f.pin(f.c.BitVectorValue(127, exponentSort))
	significand := f.pin(f.c.BitVectorValue(0, significandSort))
	value := f.pin(f.c.FloatingPointFromComponents(sign, exponent, significand))
	if kind, err := f.c.DeclarationKind(value); err != nil || kind != ast.FPAOpFP {
		t.Fatal("component value", kind, err)
	}
	if got := f.pin(f.c.SortOf(value)); got != sort {
		t.Fatal("component sort", got, sort)
	}
	if _, err := f.c.FloatingPointNaN(f.pin(f.c.BoolSort())); err == nil {
		t.Fatal("NaN accepted Boolean sort")
	}
}
