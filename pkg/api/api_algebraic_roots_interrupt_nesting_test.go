package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/util/control"
)

func TestAPIAlgebraicRootsRefusesNestedInterruptScope(t *testing.T) {
	f := newAPIAlgebraicFixture(t)
	x := algebraicPolynomialVariable(t, f, 0)
	p := algebraicPolynomialSquareMinus(t, f, x, f.two)
	scope, err := newSMTContextInterruptScope(f.c, control.EventHandlerFunc(func(control.EventCaller) {}))
	if err != nil {
		t.Fatal(err)
	}
	if roots, callErr := f.c.AlgebraicRoots(p, nil); roots != nil || !errors.Is(callErr, errSMTContextInterruptScope) {
		t.Fatal("nested algebraic roots scope", roots, callErr)
	}
	if err = scope.Close(); err != nil {
		t.Fatal(err)
	}
}
