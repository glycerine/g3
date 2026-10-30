package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/control"
)

func TestSMTContextCancelHandlerCallerAndClose(t *testing.T) {
	context := newAPIInterruptFixture(t)
	manager, err := context.Manager()
	if err != nil {
		t.Fatal(err)
	}
	handler := newSMTContextCancelHandler(manager.ResourceLimit())
	handler.HandleEvent(control.EventCallerAPIInterrupt)
	if handler.caller != control.EventCallerAPIInterrupt || !manager.ResourceLimit().CancelFlag() {
		t.Fatal("cancel handler state", handler.caller, manager.ResourceLimit().CancelFlag())
	}
	manager.ResourceLimit().ResetCancel()
	handler.close()
	handler.HandleEvent(control.EventCallerTimeout)
	if manager.ResourceLimit().CancelFlag() || handler.caller != control.EventCallerAPIInterrupt {
		t.Fatal("closed cancel handler changed state", handler.caller, manager.ResourceLimit().CancelFlag())
	}
}
