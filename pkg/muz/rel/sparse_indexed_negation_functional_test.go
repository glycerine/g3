package rel

import "testing"

func TestSparseIndexedNegationRetainsFunctionalRowsByKey(t *testing.T) {
	target := newSparseFunctionalFixture(t, []uint64{4, 8}, 1, Fact{1, 5}, Fact{2, 6})
	negative := newSparseFunctionalFixture(t, []uint64{4, 16}, 1, Fact{1, 9})
	plan, err := NewSparseIndexedNegationPlan(target, negative, []int{0}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	stats, err := plan.Execute(target, negative)
	if err != nil || stats.Removed != 1 {
		t.Fatalf("stats=%+v err=%v", stats, err)
	}
	assertSparseIndexedNegationSet(t, target, []Fact{{2, 6}})
}
