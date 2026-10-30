package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/control"
)

func TestSMTContextInterruptScopeDispatchAndRelease(t *testing.T) {
	context := newAPIInterruptFixture(t)
	manager, err := context.Manager()
	if err != nil {
		t.Fatal(err)
	}
	var callers []control.EventCaller
	scope, err := newSMTContextInterruptScope(context, control.EventHandlerFunc(func(caller control.EventCaller) {
		callers = append(callers, caller)
	}))
	if err != nil {
		t.Fatal(err)
	}
	if err = context.Interrupt(); err != nil {
		t.Fatal(err)
	}
	if len(callers) != 1 || callers[0] != control.EventCallerAPIInterrupt || !manager.ResourceLimit().CancelFlag() {
		t.Fatal("source interrupt dispatch", callers, manager.ResourceLimit().CancelFlag())
	}
	manager.ResourceLimit().ResetCancel()
	if err = scope.Close(); err != nil {
		t.Fatal(err)
	}
	if err = context.Interrupt(); err != nil {
		t.Fatal(err)
	}
	if len(callers) != 1 || !manager.ResourceLimit().CancelFlag() {
		t.Fatal("closed scope dispatch", callers, manager.ResourceLimit().CancelFlag())
	}
}
