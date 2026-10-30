package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/control"
)

func TestAPIAlgebraicRootsReleasesInterruptScope(t *testing.T) {
	f := newAPIAlgebraicFixture(t)
	x := algebraicPolynomialVariable(t, f, 0)
	p := algebraicPolynomialSquareMinus(t, f, x, f.two)
	roots, err := f.c.AlgebraicRoots(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, root := range roots {
		f.pin(root, nil)
	}
	scope, err := newSMTContextInterruptScope(f.c, control.EventHandlerFunc(func(control.EventCaller) {}))
	if err != nil {
		t.Fatal("algebraic roots leaked interrupt scope", err)
	}
	if err = scope.Close(); err != nil {
		t.Fatal(err)
	}
}
