package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/control"
)

func TestAPIOptimizeInterruptScopeDispatchesSourceCaller(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	scope, err := newAPIOptimizeInterruptScope(f.opt)
	if err != nil {
		t.Fatal(err)
	}
	manager, err := f.c.Manager()
	if err != nil {
		t.Fatal(err)
	}
	if err = f.c.Interrupt(); err != nil {
		t.Fatal(err)
	}
	if scope.handler.caller != control.EventCallerAPIInterrupt || !manager.ResourceLimit().CancelFlag() {
		t.Fatal(scope.handler.caller, manager.ResourceLimit().CancelFlag())
	}
	manager.ResourceLimit().ResetCancel()
	if err = scope.Close(); err != nil {
		t.Fatal(err)
	}
}
