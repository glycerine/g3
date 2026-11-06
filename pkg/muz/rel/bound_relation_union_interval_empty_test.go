package rel

import (
	"math/big"
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

func TestBoundRelationUnionIntervalDoesNotReviveEmptyTarget(t *testing.T) {
	target := NewBoundRelation(1, true)
	source := NewIntervalFull(1)
	astTestMust(t, source.FilterEqual(0, big.NewRat(3, 1)))

	astTestMust(t, target.UnionInterval(source))
	if !target.Empty() {
		t.Fatal("mk_union_i must leave the bound relation empty")
	}
	strict, weak, err := target.Bounds(0)
	astTestMust(t, err)
	if len(strict) != 0 || len(weak) != 0 {
		t.Fatal("mk_union_i added constraints to an empty target", strict, weak)
	}
}
