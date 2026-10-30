package api

import (
	"testing"
	"time"
)

func TestAPIPolynomialZeroTimeoutIsDisabled(t *testing.T) {
	context := newAPIInterruptFixture(t)
	manager, err := context.Manager()
	if err != nil {
		t.Fatal(err)
	}
	if err = context.SetUIntOption("timeout", 0); err != nil {
		t.Fatal(err)
	}
	scope, err := newAPIPolynomialInterruptScope(context)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if manager.ResourceLimit().CancelFlag() {
		t.Fatal("disabled polynomial timer fired")
	}
	if err = scope.Close(); err != nil {
		t.Fatal(err)
	}
}
