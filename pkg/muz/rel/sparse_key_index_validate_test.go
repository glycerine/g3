package rel

import (
	"errors"
	"testing"
)

func TestSparseKeyIndexRejectsInvalidPlansKeysAndTables(t *testing.T) {
	table := newSparseFunctionalFixture(t, []uint64{4, 8}, 1, Fact{1, 2})
	for _, columns := range [][]int{{-1}, {1}, {2}} {
		if _, err := NewSparseKeyLookupPlan(table, columns); !errors.Is(err, ErrTable) {
			t.Fatalf("columns %v: %v", columns, err)
		}
	}
	if _, err := NewSparseKeyLookupPlan(table, nil); err != nil {
		t.Fatalf("zero-column index: %v", err)
	}
	plan, err := NewSparseKeyLookupPlan(table, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, _, err := plan.Build(table)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []Fact{{}, {1, 0}} {
		if _, err := snapshot.Lookup(key); !errors.Is(err, ErrTable) {
			t.Fatalf("key %v: %v", key, err)
		}
	}
	wrongDomain := newSparseFunctionalFixture(t, []uint64{5, 8}, 1)
	wrongFunctional := newSparseFunctionalFixture(t, []uint64{4, 8}, 0)
	for _, other := range []*SparseTable{wrongDomain, wrongFunctional, nil} {
		if _, _, err := plan.Build(other); !errors.Is(err, ErrTable) {
			t.Fatalf("table %v: %v", other, err)
		}
	}
	if _, err := snapshot.Lookup(Fact{4}); !errors.Is(err, ErrTable) {
		t.Fatalf("out-of-domain key: %v", err)
	}
}
