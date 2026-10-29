package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTContextFloatingPointRelationsAndClassifications(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	sort := f.pin(f.c.FloatingPointDoubleSort())
	left := f.pin(f.c.FloatingPointZero(sort, false))
	right := f.pin(f.c.FloatingPointInfinity(sort, false))
	boolean := f.pin(f.c.BoolSort())
	relations := []struct {
		kind int32
		make func() (ast.ASTHandle, error)
	}{
		{ast.FPAOpLe, func() (ast.ASTHandle, error) { return f.c.FloatingPointLE(left, right) }},
		{ast.FPAOpLt, func() (ast.ASTHandle, error) { return f.c.FloatingPointLT(left, right) }},
		{ast.FPAOpGe, func() (ast.ASTHandle, error) { return f.c.FloatingPointGE(left, right) }},
		{ast.FPAOpGt, func() (ast.ASTHandle, error) { return f.c.FloatingPointGT(left, right) }},
		{ast.FPAOpEq, func() (ast.ASTHandle, error) { return f.c.FloatingPointEqual(left, right) }},
	}
	for _, relation := range relations {
		value := f.pin(relation.make())
		if kind, err := f.c.DeclarationKind(value); err != nil || kind != relation.kind {
			t.Fatal("relation", relation.kind, kind, err)
		}
		if got := f.pin(f.c.SortOf(value)); got != boolean {
			t.Fatal("relation sort", relation.kind, got, boolean)
		}
	}
	predicates := []struct {
		kind ast.FPAClassificationKind
		make func() (ast.ASTHandle, error)
	}{
		{ast.FPAIsNormal, func() (ast.ASTHandle, error) { return f.c.FloatingPointIsNormal(left) }},
		{ast.FPAIsSubnormal, func() (ast.ASTHandle, error) { return f.c.FloatingPointIsSubnormal(left) }},
		{ast.FPAIsZero, func() (ast.ASTHandle, error) { return f.c.FloatingPointIsZero(left) }},
		{ast.FPAIsInfinite, func() (ast.ASTHandle, error) { return f.c.FloatingPointIsInfinite(left) }},
		{ast.FPAIsNaN, func() (ast.ASTHandle, error) { return f.c.FloatingPointIsNaN(left) }},
		{ast.FPAIsNegative, func() (ast.ASTHandle, error) { return f.c.FloatingPointIsNegative(left) }},
		{ast.FPAIsPositive, func() (ast.ASTHandle, error) { return f.c.FloatingPointIsPositive(left) }},
	}
	for _, predicate := range predicates {
		value := f.pin(predicate.make())
		if kind, err := f.c.DeclarationKind(value); err != nil || kind != int32(predicate.kind) {
			t.Fatal("predicate", predicate.kind, kind, err)
		}
		if got := f.pin(f.c.SortOf(value)); got != boolean {
			t.Fatal("predicate sort", predicate.kind, got, boolean)
		}
	}
}
