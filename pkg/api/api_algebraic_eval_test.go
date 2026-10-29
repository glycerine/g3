package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIAlgebraicEvalAtIrrationalValue(t *testing.T) {
	f := newAPIAlgebraicFixture(t)
	x := algebraicPolynomialVariable(t, f, 0)
	sqrt2 := f.sqrt2(t)
	zeroPolynomial := algebraicPolynomialSquareMinus(t, f, x, f.two)
	sign, err := f.c.AlgebraicEval(zeroPolynomial, []ast.ASTHandle{sqrt2})
	astTestMust(t, err)
	if sign != 0 {
		t.Fatal("zero sign", sign)
	}
	negativePolynomial := algebraicPolynomialSquareMinus(t, f, x, f.three)
	sign, err = f.c.AlgebraicEval(negativePolynomial, []ast.ASTHandle{sqrt2})
	astTestMust(t, err)
	if sign != -1 {
		t.Fatal("negative sign", sign)
	}
	positivePolynomial := algebraicPolynomialSquareMinus(t, f, x, f.one)
	sign, err = f.c.AlgebraicEval(positivePolynomial, []ast.ASTHandle{sqrt2})
	astTestMust(t, err)
	if sign != 1 {
		t.Fatal("positive sign", sign)
	}
}
