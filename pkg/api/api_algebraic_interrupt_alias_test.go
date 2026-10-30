package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/control"
)

func TestAPIAlgebraicOperationsShareInterruptScopeAcrossAliases(t *testing.T) {
	f := newAPIAlgebraicFixture(t)
	alias := *f.c
	x := algebraicPolynomialVariable(t, f, 0)
	p := algebraicPolynomialSquareMinus(t, f, x, f.two)
	scope, err := newSMTContextInterruptScope(f.c, control.EventHandlerFunc(func(control.EventCaller) {}))
	if err != nil {
		t.Fatal(err)
	}
	if _, callErr := alias.AlgebraicEval(p, []ast.ASTHandle{f.two}); !errors.Is(callErr, errSMTContextInterruptScope) {
		t.Fatal("alias algebraic interrupt scope", callErr)
	}
	if err = scope.Close(); err != nil {
		t.Fatal(err)
	}
}
