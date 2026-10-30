package rel

import "testing"

func TestSparseNegatedJoinFiltersTargetByJoinedSources(t *testing.T) {
	target := newSparseFunctionalFixture(t, []uint64{4, 4}, 0, Fact{1, 2}, Fact{1, 3}, Fact{2, 3})
	first := newSparseFunctionalFixture(t, []uint64{4, 4}, 0, Fact{1, 0}, Fact{2, 1})
	second := newSparseFunctionalFixture(t, []uint64{4, 4}, 0, Fact{0, 2}, Fact{1, 3})
	plan, err := NewSparseNegatedJoinPlan(target, first, second, []int{0, 1}, []int{0, 3}, []int{1}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	stats, err := plan.Apply(target, first, second)
	if err != nil || stats.Removed != 2 || stats.TargetRows != 3 || stats.FirstRows != 2 || stats.SecondRows != 2 {
		t.Fatalf("stats=%#v err=%v", stats, err)
	}
	if exact, _ := target.ContainsExact(Fact{1, 3}); !exact {
		t.Fatal("unmatched target was removed")
	}
}
