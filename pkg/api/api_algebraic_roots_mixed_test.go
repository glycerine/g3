package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIAlgebraicRootsIrrationalAssignment(t *testing.T) {
	f := newAPIAlgebraicFixture(t)
	x := algebraicPolynomialVariable(t, f, 0)
	y := algebraicPolynomialVariable(t, f, 1)
	sqrt2 := f.sqrt2(t)
	p := algebraicPolynomialSquareMinus(t, f, y, x)
	roots, err := f.c.AlgebraicRoots(p, []ast.ASTHandle{sqrt2})
	astTestMust(t, err)
	if len(roots) != 2 {
		t.Fatal("root count", len(roots))
	}
	for i := range roots {
		roots[i] = f.pin(roots[i], nil)
		square, squareErr := f.c.AlgebraicPower(roots[i], 2)
		astTestMust(t, squareErr)
		square = f.pin(square, nil)
		equal, compareErr := f.c.AlgebraicEqual(square, sqrt2)
		astTestMust(t, compareErr)
		if !equal {
			t.Fatal("irrational assignment root", i)
		}
	}
}
