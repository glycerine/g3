package api

import (
	"errors"
	"testing"
)

func TestAPIOptimizeCloseAndParentLifetime(t *testing.T) {
	f := newAPIOptimizeFixture(t)
	astTestMust(t, f.opt.Assert(f.lower))
	astTestMust(t, f.opt.Close())
	if _, err := f.opt.Check(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed optimize", err)
	}
	if _, err := f.m.MkTrue(); err != nil {
		t.Fatal("optimize close retired parent manager", err)
	}
	astTestMust(t, f.opt.Close())
}
