package sat

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestSATUintSetIteratorRejectsCopiedAndClosedOwners(t *testing.T) {
	set, _ := satSolverSetOwner(t)
	copied := *set
	if _, err := copied.Begin(); !errors.Is(err, ErrSATOwner) {
		t.Fatal(err)
	}
	paramsMust(t, set.Close())
	if _, err := set.Begin(); !errors.Is(err, arena.ErrClosed) {
		t.Fatal(err)
	}
}
