package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIAlgebraicRootsRationalAssignment(t *testing.T) {
	f := newAPIAlgebraicFixture(t)
	x := algebraicPolynomialVariable(t, f, 0)
	y := algebraicPolynomialVariable(t, f, 1)
	p := algebraicPolynomialSquareMinus(t, f, y, x)
	roots, err := f.c.AlgebraicRoots(p, []ast.ASTHandle{f.two})
	astTestMust(t, err)
	if len(roots) != 2 {
		t.Fatal("root count", len(roots))
	}
	for i := range roots {
		roots[i] = f.pin(roots[i], nil)
		square, squareErr := f.c.AlgebraicPower(roots[i], 2)
		astTestMust(t, squareErr)
		square = f.pin(square, nil)
		equal, compareErr := f.c.AlgebraicEqual(square, f.two)
		astTestMust(t, compareErr)
		if !equal {
			t.Fatal("root square", i)
		}
	}
	negative, err := f.c.AlgebraicIsNegative(roots[0])
	astTestMust(t, err)
	positive, err := f.c.AlgebraicIsPositive(roots[1])
	astTestMust(t, err)
	if !negative || !positive {
		t.Fatal("ordered roots", negative, positive)
	}
}
