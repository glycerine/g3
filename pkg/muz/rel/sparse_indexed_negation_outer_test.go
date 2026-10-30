package rel

import "testing"

func TestSparseIndexedNegationUsesNegativeOuterForLargeTarget(t *testing.T) {
	facts := make([]Fact, 0, 12)
	for i := 0; i < 12; i++ {
		facts = append(facts, Fact{Element(i % 6), Element(i)})
	}
	target := newSparseFunctionalFixture(t, []uint64{6, 16}, 0, facts...)
	negative := newSparseFunctionalFixture(t, []uint64{6}, 0, Fact{1}, Fact{4})
	plan, err := NewSparseIndexedNegationPlan(target, negative, []int{0}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	stats, err := plan.Execute(target, negative)
	if err != nil {
		t.Fatal(err)
	}
	if !stats.UsedNegativeOuter || stats.Probes != 2 || stats.IndexGroups != 6 || stats.Candidates != 4 || stats.Removed != 4 {
		t.Fatalf("stats=%+v", stats)
	}
	length, _ := target.Len()
	if length != 8 {
		t.Fatalf("length=%d", length)
	}
}
