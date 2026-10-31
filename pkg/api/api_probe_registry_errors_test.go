package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestAPINamedProbeRegistryErrors(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	count, countErr := f.c.ProbeCount()
	astTestMust(t, countErr)
	if _, err := f.c.ProbeDescriptor(count); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("descriptor bound", err)
	}
	if _, err := f.c.ProbeDescription("not-a-probe"); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("unknown description", err)
	}
	if _, err := f.c.NamedProbe("not-a-probe"); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatal("unknown construction", err)
	}
	closed, err := NewSMTContext()
	astTestMust(t, err)
	astTestMust(t, closed.Close())
	if _, err = closed.ProbeNames(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed names", err)
	}
	if _, err = closed.NamedProbe("size"); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed construction", err)
	}
}
