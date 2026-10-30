package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/control"
)

func TestAPIPolynomialSubresultantsReleasesInterruptScope(t *testing.T) {
	f := newAPIPolynomialFixture(t)
	result, err := f.c.PolynomialSubresultants(f.p, f.q, f.x)
	if err != nil {
		t.Fatal(err)
	}
	if err = result.Close(); err != nil {
		t.Fatal(err)
	}
	scope, err := newSMTContextInterruptScope(f.c, control.EventHandlerFunc(func(control.EventCaller) {}))
	if err != nil {
		t.Fatal("polynomial operation leaked interrupt scope", err)
	}
	if err = scope.Close(); err != nil {
		t.Fatal(err)
	}
}
