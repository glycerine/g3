package api

import (
	"errors"
	"testing"
)

func TestAPIFixedpointObjectScopeGuards(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	if err := f.fp.Pop(); err == nil {
		t.Fatal("empty fixedpoint scope popped")
	}
	astTestMust(t, f.fp.Close())
	if err := f.fp.Push(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed push", err)
	}
	if err := f.fp.Pop(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed pop", err)
	}
}
