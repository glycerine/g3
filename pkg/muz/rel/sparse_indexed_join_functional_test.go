package rel

import "testing"

func TestSparseIndexedJoinPreservesFunctionalLayout(t *testing.T) {
	left := newSparseFunctionalFixture(t, []uint64{4, 8}, 1, Fact{1, 6}, Fact{2, 7})
	right := newSparseFunctionalFixture(t, []uint64{4, 16}, 1, Fact{1, 9}, Fact{3, 10})
	plan, err := NewSparseIndexedJoinPlan(left, right, []int{0}, []int{0}, []int{2})
	if err != nil {
		t.Fatal(err)
	}
	result, _, err := plan.Execute(left, right)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	functional, err := result.FunctionalColumns()
	if err != nil || functional != 1 {
		t.Fatalf("functional=%d, %v", functional, err)
	}
	facts, _ := result.Facts()
	// Joined layout is left-key, right-key, left-value, right-value.
	assertSparseKeyFacts(t, facts, []Fact{{1, 1, 9}})
}
