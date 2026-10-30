package api

import (
	"errors"
	"testing"
)

func TestAPIOptimizeEmptyModelCloseAndClosedOptimizeGuard(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	model, err := f.opt.Model()
	if err != nil {
		t.Fatal(err)
	}
	if err = model.Close(); err != nil {
		t.Fatal(err)
	}
	if err = model.Close(); err != nil {
		t.Fatal(err)
	}
	closed, err := f.c.NewOptimize()
	if err != nil {
		t.Fatal(err)
	}
	if err = closed.Close(); err != nil {
		t.Fatal(err)
	}
	if value, modelErr := closed.Model(); value != nil || !errors.Is(modelErr, ErrSMTContextOwner) {
		t.Fatal(value, modelErr)
	}
}
