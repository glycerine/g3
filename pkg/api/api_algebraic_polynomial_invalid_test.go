package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIAlgebraicPolynomialConsumersRejectInvalidDomains(t *testing.T) {
	f := newAPIAlgebraicFixture(t)
	truth := f.pin(f.m.MkTrue())
	if _, err := f.c.AlgebraicRoots(truth, nil); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("non-polynomial roots", err)
	}
	if _, err := f.c.AlgebraicEval(truth, nil); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("non-polynomial evaluation", err)
	}
	x := algebraicPolynomialVariable(t, f, 0)
	y := algebraicPolynomialVariable(t, f, 1)
	p := algebraicPolynomialSquareMinus(t, f, y, x)
	if _, err := f.c.AlgebraicEval(p, []ast.ASTHandle{f.two}); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("unassigned evaluation variable", err)
	}
	if _, err := f.c.AlgebraicRoots(p, []ast.ASTHandle{truth}); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("non-algebraic assignment", err)
	}
}
