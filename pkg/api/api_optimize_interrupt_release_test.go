package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/control"
)

func TestAPIOptimizeCheckReleasesInterruptScope(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	if _, err := f.opt.Minimize(f.x); err != nil {
		t.Fatal(err)
	}
	if _, err := f.opt.Check(); err != nil {
		t.Fatal(err)
	}
	scope, err := newSMTContextInterruptScope(f.c, control.EventHandlerFunc(func(control.EventCaller) {}))
	if err != nil {
		t.Fatal("optimize check leaked interrupt scope", err)
	}
	if err = scope.Close(); err != nil {
		t.Fatal(err)
	}
}
