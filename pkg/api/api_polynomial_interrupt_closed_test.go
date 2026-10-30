package api

import (
	"errors"
	"testing"
)

func TestAPIPolynomialInterruptScopeRejectsClosedContext(t *testing.T) {
	context, err := NewSMTContext()
	if err != nil {
		t.Fatal(err)
	}
	if err = context.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = newAPIPolynomialInterruptScope(context); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed polynomial interrupt scope", err)
	}
}
