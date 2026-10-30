package rel

import "testing"

func TestSparseIndexedJoinRebuildsIndexEachExecution(t *testing.T) {
	left := newSparseFunctionalFixture(t, []uint64{4}, 0, Fact{1}, Fact{2})
	right := newSparseFunctionalFixture(t, []uint64{4, 4}, 0, Fact{1, 0})
	plan, err := NewSparseIndexedJoinPlan(left, right, []int{0}, []int{0}, nil)
	if err != nil {
		t.Fatal(err)
	}
	first, firstStats, err := plan.Execute(left, right)
	if err != nil {
		t.Fatal(err)
	}
	_ = first.Close()
	if _, err := right.Add(Fact{2, 3}); err != nil {
		t.Fatal(err)
	}
	second, secondStats, err := plan.Execute(left, right)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if firstStats.Inserted != 1 || secondStats.Inserted != 2 {
		t.Fatalf("first=%+v second=%+v", firstStats, secondStats)
	}
	facts, _ := second.Facts()
	assertSparseKeyFacts(t, facts, []Fact{{1, 1, 0}, {2, 2, 3}})
}
