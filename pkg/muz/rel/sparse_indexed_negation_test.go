package rel

import "testing"

func TestSparseIndexedNegationUsesTargetOuterIndex(t *testing.T) {
	target := newSparseFunctionalFixture(t, []uint64{8, 8}, 0,
		Fact{0, 1}, Fact{1, 2}, Fact{2, 3}, Fact{3, 4})
	negative := newSparseFunctionalFixture(t, []uint64{8}, 0, Fact{1}, Fact{3})
	plan, err := NewSparseIndexedNegationPlan(target, negative, []int{0}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	stats, err := plan.Execute(target, negative)
	if err != nil {
		t.Fatal(err)
	}
	if stats.UsedNegativeOuter || stats.Probes != 4 || stats.IndexGroups != 2 || stats.Candidates != 2 || stats.Removed != 2 {
		t.Fatalf("stats=%+v", stats)
	}
	assertSparseIndexedNegationSet(t, target, []Fact{{0, 1}, {2, 3}})
}

func assertSparseIndexedNegationSet(t *testing.T, table *SparseTable, want []Fact) {
	t.Helper()
	facts, err := table.Facts()
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != len(want) {
		t.Fatalf("facts=%v want=%v", facts, want)
	}
	remaining := make(map[string]bool, len(facts))
	for _, fact := range facts {
		remaining[factKey(fact)] = true
	}
	for _, fact := range want {
		if !remaining[factKey(fact)] {
			t.Fatalf("facts=%v want=%v", facts, want)
		}
	}
}
