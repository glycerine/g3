package api

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestAPITacticRegistryCopiesBoundsAndClosedContext(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	names, err := f.c.TacticNames()
	astTestMust(t, err)
	names[0] = "changed"
	again, err := f.c.TacticNames()
	astTestMust(t, err)
	if again[0] != "qe-light" {
		t.Fatal("caller changed registry", again[0])
	}
	if _, err = f.c.TacticName(uint32(len(again))); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("registry bound", err)
	}
	closed, err := NewSMTContext()
	astTestMust(t, err)
	astTestMust(t, closed.Close())
	if _, err = closed.TacticNames(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed names", err)
	}
	if _, err = closed.TacticCount(); !errors.Is(err, ErrSMTContextOwner) {
		t.Fatal("closed count", err)
	}
}
