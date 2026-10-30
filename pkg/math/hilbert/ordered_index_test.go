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
