package api

import (
	"errors"
	"testing"
)

func TestAPIOptimizeObjectiveExpressionsOwnScopedResultsAndGuardClose(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	if err := f.opt.Push(); err != nil {
		t.Fatal(err)
	}
	if _, err := f.opt.Maximize(f.x); err != nil {
		t.Fatal(err)
	}
	refs, err := f.opt.ObjectiveExpressions()
	if err != nil || len(refs) != 1 {
		t.Fatal(len(refs), err)
	}
	if err = f.opt.Pop(); err != nil {
		t.Fatal(err)
	}
	if n, e := f.opt.NumObjectives(); e != nil || n != 0 {
		t.Fatal(n, e)
	}
	if handle, e := f.m.RefGet(refs[0]); e != nil || handle.IsNull() {
		t.Fatal(handle, e)
	}
	if err = f.m.DelRef(&refs[0]); err != nil {
		t.Fatal(err)
	}
	closed, err := f.c.NewOptimize()
	if err != nil {
		t.Fatal(err)
	}
	if err = closed.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = closed.ObjectiveExpressions(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal(err)
	}
}
