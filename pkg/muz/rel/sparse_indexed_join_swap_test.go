package rel

import "testing"

func TestSparseIndexedJoinIndexesLargerLeftTable(t *testing.T) {
	left := newSparseFunctionalFixture(t, []uint64{4, 4}, 0,
		Fact{1, 0}, Fact{1, 1}, Fact{2, 2}, Fact{3, 3})
	right := newSparseFunctionalFixture(t, []uint64{4}, 0, Fact{1}, Fact{3})
	plan, err := NewSparseIndexedJoinPlan(left, right, []int{0}, []int{0}, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, stats, err := plan.Execute(left, right)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	if !stats.IndexedLeft || stats.Probes != 2 || stats.IndexGroups != 3 {
		t.Fatalf("stats=%+v", stats)
	}
	facts, _ := result.Facts()
	assertSparseKeyFacts(t, facts, []Fact{{1, 0, 1}, {1, 1, 1}, {3, 3, 3}})
}
