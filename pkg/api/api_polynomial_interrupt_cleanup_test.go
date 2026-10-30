package api

import (
	"testing"
	"time"
)

func TestAPIPolynomialInterruptScopeCloseStopsTimer(t *testing.T) {
	context := newAPIInterruptFixture(t)
	manager, err := context.Manager()
	if err != nil {
		t.Fatal(err)
	}
	if err = context.SetUIntOption("timeout", 20); err != nil {
		t.Fatal(err)
	}
	scope, err := newAPIPolynomialInterruptScope(context)
	if err != nil {
		t.Fatal(err)
	}
	if err = scope.Close(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(40 * time.Millisecond)
	if manager.ResourceLimit().CancelFlag() {
		t.Fatal("closed polynomial timer fired")
	}
}
