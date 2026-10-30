package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/control"
)

func TestAPIAlgebraicEvalValidatesBeforeInterruptScope(t *testing.T) {
	f := newAPIAlgebraicFixture(t)
	x := algebraicPolynomialVariable(t, f, 0)
	p := algebraicPolynomialSquareMinus(t, f, x, f.two)
	scope, err := newSMTContextInterruptScope(f.c, control.EventHandlerFunc(func(control.EventCaller) {}))
	if err != nil {
		t.Fatal(err)
	}
	if sign, callErr := f.c.AlgebraicEval(p, nil); sign != 0 || !errors.Is(callErr, ast.ErrASTDomain) {
		t.Fatal("algebraic eval validation order", sign, callErr)
	}
	if err = scope.Close(); err != nil {
		t.Fatal(err)
	}
}
