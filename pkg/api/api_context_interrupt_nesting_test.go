package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/util/control"
)

func TestSMTContextInterruptScopeRejectsNesting(t *testing.T) {
	context := newAPIInterruptFixture(t)
	first, err := newSMTContextInterruptScope(context, control.EventHandlerFunc(func(control.EventCaller) {}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = newSMTContextInterruptScope(context, control.EventHandlerFunc(func(control.EventCaller) {})); !errors.Is(err, errSMTContextInterruptScope) {
		t.Fatal("nested interrupt scope", err)
	}
	if err = first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := newSMTContextInterruptScope(context, control.EventHandlerFunc(func(control.EventCaller) {}))
	if err != nil {
		t.Fatal("scope was not released", err)
	}
	if err = second.Close(); err != nil {
		t.Fatal(err)
	}
}
