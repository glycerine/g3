package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/util/control"
)

func TestSMTContextSimplifyScopesInterruptHandler(t *testing.T) {
	context := newAPIInterruptFixture(t)
	manager, err := context.Manager()
	if err != nil {
		t.Fatal(err)
	}
	truth, err := manager.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	result, err := context.Simplify(truth)
	if err != nil {
		t.Fatal(err)
	}
	if err = manager.DelRef(&result); err != nil {
		t.Fatal(err)
	}
	scope, err := newSMTContextInterruptScope(context, control.EventHandlerFunc(func(control.EventCaller) {}))
	if err != nil {
		t.Fatal("simplify leaked interrupt scope", err)
	}
	if _, err = context.Simplify(truth); !errors.Is(err, errSMTContextInterruptScope) {
		t.Fatal("simplify entered nested interrupt scope", err)
	}
	if err = scope.Close(); err != nil {
		t.Fatal(err)
	}
}
