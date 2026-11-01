package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestAPIAlgebraicRejectsInvalidOperands(t *testing.T) {
	f := newAPIAlgebraicFixture(t)
	truth := f.pin(f.m.MkTrue())
	if _, err := f.c.AlgebraicAdd(truth, f.one); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("non-value arithmetic", err)
	}
	if _, err := f.c.AlgebraicDiv(f.one, f.zero); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("division by zero", err)
	}
	negative, err := f.c.AlgebraicSub(f.zero, f.one)
	astTestMust(t, err)
	negative = f.pin(negative, nil)
	if _, err = f.c.AlgebraicRoot(negative, 2); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("even root of negative", err)
	}
}

func TestAPIAlgebraicRejectsForeignContextOperands(t *testing.T) {
	left := newAPIAlgebraicFixture(t)
	right := newAPIAlgebraicFixture(t)
	if _, err := left.c.AlgebraicAdd(left.one, right.one); !errors.Is(err, ast.ErrASTHandle) {
		t.Fatal("foreign algebraic operand error", err)
	}
	if text, err := left.c.NumeralString(left.one); err != nil || text != "1" {
		t.Fatal("foreign operand rejection changed the owning context", text, err)
	}
}
