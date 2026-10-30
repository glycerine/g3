package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/util/control"
)

func TestAPIPolynomialSubresultantsRefusesNestedInterruptScope(t *testing.T) {
	f := newAPIPolynomialFixture(t)
	scope, err := newSMTContextInterruptScope(f.c, control.EventHandlerFunc(func(control.EventCaller) {}))
	if err != nil {
		t.Fatal(err)
	}
	if result, callErr := f.c.PolynomialSubresultants(f.p, f.q, f.x); result != nil || !errors.Is(callErr, errSMTContextInterruptScope) {
		t.Fatal("nested polynomial interrupt scope", result, callErr)
	}
	if err = scope.Close(); err != nil {
		t.Fatal(err)
	}
}
