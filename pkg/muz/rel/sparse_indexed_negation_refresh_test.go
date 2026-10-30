package rel

import "testing"

func TestSparseIndexedNegationRebuildsAfterNegativeMutation(t *testing.T) {
	target := newSparseFunctionalFixture(t, []uint64{4}, 0, Fact{1}, Fact{2})
	negative := newSparseFunctionalFixture(t, []uint64{4}, 0, Fact{3})
	plan, err := NewSparseIndexedNegationPlan(target, negative, []int{0}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	first, err := plan.Execute(target, negative)
	if err != nil || first.Removed != 0 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	if _, err := negative.Add(Fact{2}); err != nil {
		t.Fatal(err)
	}
	second, err := plan.Execute(target, negative)
	if err != nil || second.Removed != 1 {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	assertSparseIndexedNegationSet(t, target, []Fact{{1}})
}
