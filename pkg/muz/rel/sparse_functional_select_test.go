package rel

import "testing"

func TestSparseFunctionalSelectProjectTracksBoundary(t *testing.T) {
	table := newSparseFunctionalFixture(t, []uint64{4, 4, 8}, 1, Fact{1, 2, 3}, Fact{0, 2, 5})
	keyProjection, err := table.SelectEqualProjectFunctional(1, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer keyProjection.Close()
	if functional, _ := keyProjection.FunctionalColumns(); functional != 0 {
		t.Fatalf("key projection functional=%d", functional)
	}
	if exact, _ := keyProjection.ContainsExact(Fact{2, 3}); !exact {
		t.Fatal("selected key row missing")
	}
	empty, err := table.SelectEqualProjectFunctional(4, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer empty.Close()
	if count, _ := empty.Len(); count != 0 {
		t.Fatalf("out-of-domain selection produced %d rows, want 0", count)
	}
	if _, err := table.SelectEqualProjectFunctional(3, 2); err != ErrTable {
		t.Fatalf("functional-column selection error=%v, want %v", err, ErrTable)
	}
	if exact, _ := table.ContainsExact(Fact{1, 2, 3}); !exact {
		t.Fatal("rejected functional-column selection mutated the source table")
	}
}
