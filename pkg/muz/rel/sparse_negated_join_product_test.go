package rel

import "testing"

func TestSparseNegatedJoinWithoutTargetMappingsUsesSourceNonemptiness(t *testing.T) {
	target := newSparseFunctionalFixture(t, []uint64{4}, 0, Fact{0}, Fact{1})
	first := newSparseFunctionalFixture(t, []uint64{4}, 0, Fact{2})
	second := newSparseFunctionalFixture(t, []uint64{4}, 0, Fact{3})
	plan, err := NewSparseNegatedJoinPlan(target, first, second, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	stats, err := plan.Apply(target, first, second)
	if err != nil || stats.Removed != 2 {
		t.Fatalf("stats=%#v err=%v", stats, err)
	}
	if length, _ := target.Len(); length != 0 {
		t.Fatalf("remaining=%d", length)
	}
	emptySecond := newSparseFunctionalFixture(t, []uint64{4}, 0)
	target = newSparseFunctionalFixture(t, []uint64{4}, 0, Fact{0})
	plan, _ = NewSparseNegatedJoinPlan(target, first, emptySecond, nil, nil, nil, nil)
	stats, err = plan.Apply(target, first, emptySecond)
	if err != nil || stats.Removed != 0 {
		t.Fatalf("empty source stats=%#v err=%v", stats, err)
	}
}
