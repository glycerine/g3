package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

type apiPolynomialFixture struct {
	*apiAlgebraicFixture
	x, y, p, q ast.ASTHandle
}

func newAPIPolynomialFixture(t *testing.T) *apiPolynomialFixture {
	f := newAPIAlgebraicFixture(t)
	x := f.arithmeticVariable("api-polynomial-x", false)
	y := f.arithmeticVariable("api-polynomial-y", false)
	xSquared := f.pin(f.c.ArithmeticMul(x, x))
	p := f.pin(f.c.ArithmeticSub(xSquared, f.two))
	q := f.pin(f.c.ArithmeticSub(x, f.one))
	return &apiPolynomialFixture{apiAlgebraicFixture: f, x: x, y: y, p: p, q: q}
}

func polynomialResultStrings(t *testing.T, f *apiPolynomialFixture, result *APIPolynomialSubresultants) []string {
	t.Helper()
	printer, err := f.c.NewASTPrinter(APIASTPrintSMT2)
	astTestMust(t, err)
	values, err := result.Values()
	astTestMust(t, err)
	strings := make([]string, len(values))
	for i := range values {
		strings[i], err = printer.String(values[i])
		astTestMust(t, err)
	}
	return strings
}
