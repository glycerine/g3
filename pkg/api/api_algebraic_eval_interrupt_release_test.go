package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/control"
)

func TestAPIAlgebraicEvalReleasesInterruptScope(t *testing.T) {
	f := newAPIAlgebraicFixture(t)
	x := algebraicPolynomialVariable(t, f, 0)
	p := algebraicPolynomialSquareMinus(t, f, x, f.two)
	if _, err := f.c.AlgebraicEval(p, []ast.ASTHandle{f.two}); err != nil {
		t.Fatal(err)
	}
	scope, err := newSMTContextInterruptScope(f.c, control.EventHandlerFunc(func(control.EventCaller) {}))
	if err != nil {
		t.Fatal("algebraic eval leaked interrupt scope", err)
	}
	if err = scope.Close(); err != nil {
		t.Fatal(err)
	}
}
