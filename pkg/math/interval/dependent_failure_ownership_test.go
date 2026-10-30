package interval

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestDependentIntervalForeignClosedAndWithoutDeps(t *testing.T) {
	f := newDependentIntervalFixture(t)
	a := f.interval("1", "2", false, false, 1, 2)
	b := f.interval("3", "4", false, false, 3, 4)
	result := f.interval("0", "0", false, false, 8, 9)
	if err := result.Add(a, b, false); err != nil {
		t.Fatal(err)
	}
	if got := result.LowerDependencies(); len(got) != 1 || got[0] != 8 {
		t.Fatal(got)
	}
	foreign := newDependentIntervalFixture(t).interval("1", "1", false, false, 5, 5)
	if err := result.Add(a, foreign, true); !errors.Is(err, arena.ErrInvalidSlot) {
		t.Fatal(err)
	}
	closed := f.empty()
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	if err := result.CopyFrom(closed, true); !errors.Is(err, arena.ErrClosed) {
		t.Fatal(err)
	}
}
