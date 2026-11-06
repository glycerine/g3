package rel

import (
	"testing"
)

func TestBoundRelationUnionFromEmptyIntervalUsesResetPayload(t *testing.T) {
	bound := NewBoundRelation(2, false)
	astTestMust(t, bound.AddLess(0, 1))
	astTestMust(t, bound.AddLessEqual(1, 0))

	// bound_relation::mk_union_i reads src[i].sup() without checking
	// src.empty(). An empty interval relation has reset, unbounded payloads,
	// so the source path clears every bound on each representative column.
	emptyIntervals := NewIntervalEmpty(2)
	if !emptyIntervals.Empty() {
		t.Fatal("precondition: interval relation should remain empty")
	}
	if err := bound.UnionInterval(emptyIntervals); err != nil {
		t.Fatal(err)
	}
	for column := uint32(0); column < 2; column++ {
		strict, weak, err := bound.Bounds(column)
		astTestMust(t, err)
		if len(strict) != 0 || len(weak) != 0 {
			t.Fatalf("empty interval payload did not clear column %d bounds: lt=%v le=%v", column, strict, weak)
		}
	}
}
