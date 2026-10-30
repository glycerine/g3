package api

import (
	"errors"
	"testing"
)

func TestAPIFixedpointCloseAndParentLifetime(t *testing.T) {
	f := newAPIFixedpointFixture(t)
	astTestMust(t, f.fp.Close())
	if _, err := f.fp.Query(f.p); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed fixedpoint", err)
	}
	if _, err := f.m.MkTrue(); err != nil {
		t.Fatal("fixedpoint close retired parent manager", err)
	}
	astTestMust(t, f.fp.Close())
}
