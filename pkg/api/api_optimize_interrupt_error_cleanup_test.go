package api

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAPIOptimizeCheckErrorStillReleasesInterruptScope(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	// context::validate_assumptions warns and the check is l_undef; the
	// interrupt scope must be released on that early return too
	// (AUDIT.md smtcore_01-F9).
	if status, err := f.opt.Check(f.x); err != nil || status != lbool.Undef {
		t.Fatal("non-Boolean assumption must give l_undef without an error", status, err)
	}
	scope, err := newSMTContextInterruptScope(f.c, control.EventHandlerFunc(func(control.EventCaller) {}))
	if err != nil {
		t.Fatal("failed check leaked interrupt scope", err)
	}
	if err = scope.Close(); err != nil {
		t.Fatal(err)
	}
}
