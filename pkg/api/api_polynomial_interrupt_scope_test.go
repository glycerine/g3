package api

import (
	"testing"
	"time"

	"github.com/glycerine/g3/pkg/util/control"
)

func TestAPIPolynomialInterruptScopeTimesOutWithSourceCaller(t *testing.T) {
	context := newAPIInterruptFixture(t)
	limit, err := context.polynomialLimit()
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
	for !limit.CancelFlag() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	manager, err := context.Manager()
	astTestMust(t, err)
	if manager.ResourceLimit().CancelFlag() {
		t.Fatal("polynomial timeout canceled the independent AST limit")
	}
	if !limit.CancelFlag() || scope.handler.caller != control.EventCallerTimeout {
		t.Fatal("polynomial timeout", limit.CancelFlag(), scope.handler.caller)
	}
}
