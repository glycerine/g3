package rel

import "testing"

func TestSparseIndexedJoinUsesKeyGroupsInSourceOrder(t *testing.T) {
	left := newSparseFunctionalFixture(t, []uint64{4, 4}, 0, Fact{1, 0}, Fact{2, 1})
	right := newSparseFunctionalFixture(t, []uint64{4, 4}, 0, Fact{1, 2}, Fact{1, 3}, Fact{3, 0})
	plan, err := NewSparseIndexedJoinPlan(left, right, []int{0}, []int{0}, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, stats, err := plan.Execute(left, right)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	if stats.IndexedLeft || stats.Probes != 2 || stats.IndexGroups != 2 || stats.CandidateRows != 2 || stats.Inserted != 2 {
		t.Fatalf("stats=%+v", stats)
	}
	facts, _ := result.Facts()
	assertSparseKeyFacts(t, facts, []Fact{{1, 0, 1, 2}, {1, 0, 1, 3}})
}
