package hilbert

import "testing"

func TestOrderedIndexUsesSourceDominanceAndNegativeWeightPartition(t *testing.T) {
	b := New(nil)
	index := &orderedIndex{owner: b}
	existing := &vector{values: []int64{1, 1}, weight: -2}
	index.insert(existing)
	if index.subsumes(&vector{values: []int64{2, 2}, weight: -1}) {
		t.Fatal("different negative weight must not subsume")
	}
	if !index.subsumes(&vector{values: []int64{2, 2}, weight: -2}) {
		t.Fatal("equal negative weight and dominated coordinates should subsume")
	}
}

// AUDIT.md math_misc_01-F3: hilbert_basis::index keeps positive, zero and
// per-weight negative entries apart, so a positive candidate is never
// pruned by a zero-weight vector it dominates.
func TestIndexDoesNotPrunePositiveByZeroWeight(t *testing.T) {
	b := New(nil)
	idx := &orderedIndex{owner: b}
	idx.insert(&vector{values: []int64{0, 1, 0}, weight: 0})
	if idx.subsumes(&vector{values: []int64{0, 1, 1}, weight: 2}) {
		t.Fatal("positive candidate pruned by a zero-weight entry")
	}
	if !idx.subsumes(&vector{values: []int64{0, 1, 1}, weight: 0}) {
		t.Fatal("zero-weight candidate must be subsumed by a dominated zero-weight entry")
	}
	idx.insert(&vector{values: []int64{0, 1, 0}, weight: 1})
	if !idx.subsumes(&vector{values: []int64{0, 1, 1}, weight: 2}) {
		t.Fatal("positive candidate must be subsumed by a dominated positive entry")
	}
}
