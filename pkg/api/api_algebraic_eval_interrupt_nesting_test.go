package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/control"
)

func TestAPIAlgebraicEvalRefusesNestedInterruptScope(t *testing.T) {
	f := newAPIAlgebraicFixture(t)
	x := algebraicPolynomialVariable(t, f, 0)
	p := algebraicPolynomialSquareMinus(t, f, x, f.two)
	scope, err := newSMTContextInterruptScope(f.c, control.EventHandlerFunc(func(control.EventCaller) {}))
	if err != nil {
		t.Fatal(err)
	}
	if sign, callErr := f.c.AlgebraicEval(p, []ast.ASTHandle{f.two}); sign != 0 || !errors.Is(callErr, errSMTContextInterruptScope) {
		t.Fatal("nested algebraic eval scope", sign, callErr)
	}
	if err = scope.Close(); err != nil {
		t.Fatal(err)
	}
}
