package rel

import "testing"

func TestSparseIndexedNegationProductClearsOnlyForNonemptyNegative(t *testing.T) {
	target := newSparseFunctionalFixture(t, []uint64{4}, 0, Fact{0}, Fact{1})
	empty := newSparseFunctionalFixture(t, []uint64{8}, 0)
	plan, err := NewSparseIndexedNegationPlan(target, empty, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	stats, err := plan.Execute(target, empty)
	if err != nil || stats.Removed != 0 || stats.ClearedProduct {
		t.Fatalf("empty stats=%+v err=%v", stats, err)
	}
	negative := newSparseFunctionalFixture(t, []uint64{8}, 0, Fact{7})
	plan, err = NewSparseIndexedNegationPlan(target, negative, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	stats, err = plan.Execute(target, negative)
	if err != nil || !stats.ClearedProduct || stats.Removed != 2 {
		t.Fatalf("nonempty stats=%+v err=%v", stats, err)
	}
	length, _ := target.Len()
	if length != 0 {
		t.Fatalf("length=%d", length)
	}
}
