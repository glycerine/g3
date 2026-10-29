package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func algebraicPolynomialVariable(t *testing.T, f *apiAlgebraicFixture, index uint32) ast.ASTHandle {
	t.Helper()
	realSort := f.pin(f.c.RealSort())
	return f.pin(f.m.MkVar(index, realSort))
}

func algebraicPolynomialSquareMinus(t *testing.T, f *apiAlgebraicFixture, variable, rhs ast.ASTHandle) ast.ASTHandle {
	t.Helper()
	square := f.pin(f.c.ArithmeticMul(variable, variable))
	return f.pin(f.c.ArithmeticSub(square, rhs))
}
