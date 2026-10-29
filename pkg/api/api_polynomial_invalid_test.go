package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIPolynomialSubresultantsRejectsNonPolynomialInput(t *testing.T) {
	f := newAPIPolynomialFixture(t)
	truth := f.pin(f.m.MkTrue())
	if result, err := f.c.PolynomialSubresultants(truth, f.q, f.x); result != nil || !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("left non-polynomial", result, err)
	}
	if result, err := f.c.PolynomialSubresultants(f.p, truth, f.x); result != nil || !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("right non-polynomial", result, err)
	}
}
