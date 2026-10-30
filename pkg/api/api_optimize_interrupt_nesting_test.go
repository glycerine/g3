package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIOptimizeCheckRejectsNestedInterruptScope(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	scope, err := newSMTContextInterruptScope(f.c, control.EventHandlerFunc(func(control.EventCaller) {}))
	if err != nil {
		t.Fatal(err)
	}
	defer scope.Close()
	status, err := f.opt.Check()
	if status != lbool.Undef || !errors.Is(err, errSMTContextInterruptScope) {
		t.Fatal(status, err)
	}
}
