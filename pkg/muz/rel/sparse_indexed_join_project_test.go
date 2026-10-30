package rel

import "testing"

func TestSparseIndexedJoinProjectsAndDeduplicates(t *testing.T) {
	left := newSparseFunctionalFixture(t, []uint64{4, 4}, 0, Fact{1, 0}, Fact{1, 1})
	right := newSparseFunctionalFixture(t, []uint64{4, 4}, 0, Fact{1, 2}, Fact{1, 3})
	plan, err := NewSparseIndexedJoinPlan(left, right, []int{0}, []int{0}, []int{1, 3})
	if err != nil {
		t.Fatal(err)
	}
	result, stats, err := plan.Execute(left, right)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	if stats.CandidateRows != 4 || stats.Inserted != 1 {
		t.Fatalf("stats=%+v", stats)
	}
	facts, _ := result.Facts()
	assertSparseKeyFacts(t, facts, []Fact{{1, 1}})
}

func TestSparseIndexedJoinProductKeepsSmallOperandInner(t *testing.T) {
	left := newSparseFunctionalFixture(t, []uint64{4}, 0, Fact{0})
	right := newSparseFunctionalFixture(t, []uint64{4}, 0, Fact{1}, Fact{2})
	plan, err := NewSparseIndexedJoinPlan(left, right, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, stats, err := plan.Execute(left, right)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	if !stats.Product || stats.Probes != 2 || stats.IndexGroups != 0 {
		t.Fatalf("stats=%+v", stats)
	}
	facts, _ := result.Facts()
	assertSparseKeyFacts(t, facts, []Fact{{0, 1}, {0, 2}})
}
