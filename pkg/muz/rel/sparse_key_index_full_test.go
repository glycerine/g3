package rel

import "testing"

func TestSparseKeyIndexFullSignatureSpecialization(t *testing.T) {
	table := newSparseFunctionalFixture(t, []uint64{3, 4}, 0,
		Fact{2, 1}, Fact{0, 3}, Fact{1, 2})
	plan, err := NewSparseKeyLookupPlan(table, []int{0, 1})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, stats, err := plan.Build(table)
	if err != nil {
		t.Fatal(err)
	}
	if !stats.FullSignature || stats.Groups != 3 || stats.Rows != 3 {
		t.Fatalf("stats=%+v", stats)
	}
	for _, fact := range []Fact{{2, 1}, {0, 3}, {1, 2}} {
		found, err := snapshot.Contains(fact)
		if err != nil || !found {
			t.Fatalf("contains(%v)=%v, %v", fact, found, err)
		}
	}
	missing, err := snapshot.Lookup(Fact{2, 3})
	if err != nil || missing == nil || len(missing) != 0 {
		t.Fatalf("missing=%v, %v", missing, err)
	}
}
