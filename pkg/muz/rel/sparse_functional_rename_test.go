package rel

import (
	"errors"
	"testing"
)

func TestSparseFunctionalRenamePreservesBoundaryAndRejectsCrossing(t *testing.T) {
	table := newSparseFunctionalFixture(t, []uint64{4, 8, 16, 32}, 2, Fact{1, 2, 3, 4})
	renamed, err := table.RenameFunctional([]int{0, 1})
	if err != nil {
		t.Fatal(err)
	}
	defer renamed.Close()
	if functional, _ := renamed.FunctionalColumns(); functional != 2 {
		t.Fatalf("functional=%d", functional)
	}
	if exact, _ := renamed.ContainsExact(Fact{2, 1, 3, 4}); !exact {
		t.Fatal("renamed fact missing")
	}
	if _, err := table.RenameFunctional([]int{1, 2}); !errors.Is(err, ErrTable) {
		t.Fatalf("cross-boundary rename=%v", err)
	}
	if _, err := table.RenameFunctional([]int{0, 0}); !errors.Is(err, ErrTable) {
		t.Fatalf("duplicate rename=%v", err)
	}
}
