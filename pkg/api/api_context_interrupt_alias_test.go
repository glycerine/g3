package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/control"
)

func TestSMTContextInterruptAliasesShareScope(t *testing.T) {
	context := newAPIInterruptFixture(t)
	alias := *context
	called := false
	scope, err := newSMTContextInterruptScope(context, control.EventHandlerFunc(func(caller control.EventCaller) {
		called = caller == control.EventCallerAPIInterrupt
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer scope.Close()
	if err = alias.Interrupt(); err != nil || !called {
		t.Fatal("alias interrupt", called, err)
	}
}
