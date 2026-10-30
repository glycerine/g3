package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/control"
)

func TestSMTContextParameterizedSimplifyReleasesInterruptScope(t *testing.T) {
	context := newAPIInterruptFixture(t)
	manager, err := context.Manager()
	if err != nil {
		t.Fatal(err)
	}
	truth, err := manager.MkTrue()
	if err != nil {
		t.Fatal(err)
	}
	parameters, err := context.NewParameterSet()
	if err != nil {
		t.Fatal(err)
	}
	defer parameters.Close()
	result, err := context.SimplifyWithParameters(truth, parameters)
	if err != nil {
		t.Fatal(err)
	}
	if err = manager.DelRef(&result); err != nil {
		t.Fatal(err)
	}
	scope, err := newSMTContextInterruptScope(context, control.EventHandlerFunc(func(control.EventCaller) {}))
	if err != nil {
		t.Fatal("parameterized simplify leaked interrupt scope", err)
	}
	if err = scope.Close(); err != nil {
		t.Fatal(err)
	}
}
