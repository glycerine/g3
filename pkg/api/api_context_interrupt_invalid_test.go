package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/util/control"
)

func TestSMTContextInterruptScopeRejectsInvalidOwnersAndHandlers(t *testing.T) {
	context := newAPIInterruptFixture(t)
	if _, err := newSMTContextInterruptScope(context, nil); !errors.Is(err, errSMTContextInterruptScope) {
		t.Fatal("nil handler", err)
	}
	var nilFunction control.EventHandlerFunc
	if _, err := newSMTContextInterruptScope(context, nilFunction); !errors.Is(err, errSMTContextInterruptScope) {
		t.Fatal("nil function handler", err)
	}
	closed, err := NewSMTContext()
	if err != nil {
		t.Fatal(err)
	}
	if err = closed.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = newSMTContextInterruptScope(closed, control.EventHandlerFunc(func(control.EventCaller) {})); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed context scope", err)
	}
}
