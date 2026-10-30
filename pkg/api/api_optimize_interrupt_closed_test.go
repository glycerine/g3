package api

import (
	"errors"
	"testing"
)

func TestAPIOptimizeInterruptScopeRejectsClosedObject(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	closed, err := f.c.NewOptimize()
	if err != nil {
		t.Fatal(err)
	}
	if err = closed.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = newAPIOptimizeInterruptScope(closed); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal(err)
	}
}
