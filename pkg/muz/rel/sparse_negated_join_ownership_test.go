package rel

import "testing"

func TestSparseNegatedJoinPlanOwnsColumnsAndReusesSchemas(t *testing.T) {
	target := newSparseFunctionalFixture(t, []uint64{4}, 0, Fact{1})
	first := newSparseFunctionalFixture(t, []uint64{4}, 0, Fact{1})
	second := newSparseFunctionalFixture(t, []uint64{4}, 0, Fact{1})
	targetColumns := []int{0}
	sourceColumns := []int{0}
	plan, err := NewSparseNegatedJoinPlan(target, first, second, targetColumns, sourceColumns, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	targetColumns[0], sourceColumns[0] = 9, 9
	clone := plan.Clone()
	columns, sources, _, _ := clone.Columns()
	columns[0], sources[0] = 8, 8
	again, sourceAgain, _, _ := clone.Columns()
	if again[0] != 0 || sourceAgain[0] != 0 {
		t.Fatal("plan columns escaped ownership")
	}
	target2 := newSparseFunctionalFixture(t, []uint64{4}, 0, Fact{1}, Fact{2})
	first2 := newSparseFunctionalFixture(t, []uint64{4}, 0, Fact{1})
	second2 := newSparseFunctionalFixture(t, []uint64{4}, 0, Fact{0})
	stats, err := clone.Apply(target2, first2, second2)
	if err != nil || stats.Removed != 1 {
		t.Fatalf("stats=%#v err=%v", stats, err)
	}
}
