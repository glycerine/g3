package api

import (
	"testing"
	"time"

	"github.com/glycerine/g3/pkg/util/control"
)

func TestAPIPolynomialInterruptScopeTimesOutWithSourceCaller(t *testing.T) {
	context := newAPIInterruptFixture(t)
	manager, err := context.Manager()
	if err != nil {
		t.Fatal(err)
	}
	if err = context.SetUIntOption("timeout", 1); err != nil {
		t.Fatal(err)
	}
	scope, err := newAPIPolynomialInterruptScope(context)
	if err != nil {
		t.Fatal(err)
	}
	defer scope.Close()
	deadline := time.Now().Add(time.Second)
	for !manager.ResourceLimit().CancelFlag() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !manager.ResourceLimit().CancelFlag() || scope.handler.caller != control.EventCallerTimeout {
		t.Fatal("polynomial timeout", manager.ResourceLimit().CancelFlag(), scope.handler.caller)
	}
}
