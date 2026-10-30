package rel

import "testing"

func TestSparseNegatedJoinRetainsSourceFunctionalColumnConsumer(t *testing.T) {
	target := newSparseFunctionalFixture(t, []uint64{4, 4}, 1, Fact{1, 2}, Fact{2, 3})
	first := newSparseFunctionalFixture(t, []uint64{4, 4}, 1, Fact{1, 2})
	second := newSparseFunctionalFixture(t, []uint64{4, 4}, 1, Fact{0, 2})
	plan, err := NewSparseNegatedJoinPlan(target, first, second, []int{0, 1}, []int{0, 3}, []int{1}, []int{1})
	if err != nil {
		t.Fatal(err)
	}
	stats, err := plan.Apply(target, first, second)
	if err != nil || stats.Removed != 1 {
		t.Fatalf("stats=%#v err=%v", stats, err)
	}
	if exact, _ := target.ContainsExact(Fact{2, 3}); !exact {
		t.Fatal("unmatched functional row removed")
	}
}
