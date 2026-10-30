package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/control"
)

func TestAPIOptimizeCheckErrorStillReleasesInterruptScope(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	if _, err := f.opt.Check(f.x); err == nil {
		t.Fatal("non-Boolean assumption accepted")
	}
	scope, err := newSMTContextInterruptScope(f.c, control.EventHandlerFunc(func(control.EventCaller) {}))
	if err != nil {
		t.Fatal("failed check leaked interrupt scope", err)
	}
	if err = scope.Close(); err != nil {
		t.Fatal(err)
	}
}
