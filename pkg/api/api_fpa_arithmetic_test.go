package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextFloatingPointArithmeticConstructors(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	sort := f.pin(f.c.FloatingPointSingleSort())
	rounding := f.pin(f.c.FloatingPointRoundNearestEven())
	left := f.pin(f.c.FloatingPointZero(sort, false))
	right := f.pin(f.c.FloatingPointInfinity(sort, false))
	third := f.pin(f.c.FloatingPointZero(sort, true))
	tests := []struct {
		kind int32
		make func() (ast.ASTHandle, error)
	}{
		{ast.FPAOpAbs, func() (ast.ASTHandle, error) { return f.c.FloatingPointAbs(left) }},
		{ast.FPAOpNeg, func() (ast.ASTHandle, error) { return f.c.FloatingPointNeg(left) }},
		{ast.FPAOpAdd, func() (ast.ASTHandle, error) { return f.c.FloatingPointAdd(rounding, left, right) }},
		{ast.FPAOpSub, func() (ast.ASTHandle, error) { return f.c.FloatingPointSub(rounding, left, right) }},
		{ast.FPAOpMul, func() (ast.ASTHandle, error) { return f.c.FloatingPointMul(rounding, left, right) }},
		{ast.FPAOpDiv, func() (ast.ASTHandle, error) { return f.c.FloatingPointDiv(rounding, left, right) }},
		{ast.FPAOpFMA, func() (ast.ASTHandle, error) { return f.c.FloatingPointFusedMultiplyAdd(rounding, left, right, third) }},
		{ast.FPAOpSqrt, func() (ast.ASTHandle, error) { return f.c.FloatingPointSqrt(rounding, left) }},
		{ast.FPAOpRem, func() (ast.ASTHandle, error) { return f.c.FloatingPointRemainder(left, right) }},
		{ast.FPAOpRoundToIntegral, func() (ast.ASTHandle, error) { return f.c.FloatingPointRoundToIntegral(rounding, left) }},
		{ast.FPAOpMin, func() (ast.ASTHandle, error) { return f.c.FloatingPointMin(left, right) }},
		{ast.FPAOpMax, func() (ast.ASTHandle, error) { return f.c.FloatingPointMax(left, right) }},
	}
	for _, test := range tests {
		value := f.pin(test.make())
		if kind, err := f.c.DeclarationKind(value); err != nil || kind != test.kind {
			t.Fatalf("operator %d: kind %d, %v", test.kind, kind, err)
		}
		if got := f.pin(f.c.SortOf(value)); got != sort {
			t.Fatalf("operator %d sort: %v != %v", test.kind, got, sort)
		}
	}
	otherSort := f.pin(f.c.FloatingPointHalfSort())
	other := f.pin(f.c.FloatingPointZero(otherSort, false))
	if _, err := f.c.FloatingPointAdd(rounding, left, other); err == nil {
		t.Fatal("mixed floating-point formats accepted")
	}
	if _, err := f.c.FloatingPointAdd(left, left, right); err == nil {
		t.Fatal("floating-point value accepted as rounding mode")
	}
}
