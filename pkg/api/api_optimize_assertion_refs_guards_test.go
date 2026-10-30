package api

import (
	"errors"
	"testing"
)

func TestAPIOptimizeAssertionRefsEmptyAndClosedGuards(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	refs, err := f.opt.AssertionRefs()
	if err != nil || len(refs) != 0 {
		t.Fatal(refs, err)
	}
	closed, err := f.c.NewOptimize()
	if err != nil {
		t.Fatal(err)
	}
	if err = closed.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = closed.AssertionRefs(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal(err)
	}
}
