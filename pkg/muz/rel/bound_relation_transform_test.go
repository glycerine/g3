package rel

import (
	"reflect"
	"testing"
)

func TestBoundRelationJoinAndProjection(t *testing.T) {
	left, right := NewBoundRelation(2, false), NewBoundRelation(2, false)
	astTestMust(t, left.AddLess(0, 1))
	astTestMust(t, right.AddLessEqual(0, 1))
	joined, err := JoinBoundRelations(left, right, nil, nil)
	astTestMust(t, err)
	strict, weak, err := joined.Bounds(0)
	astTestMust(t, err)
	if !reflect.DeepEqual(strict, []uint32{1}) || len(weak) != 0 {
		t.Fatal("left joined bounds", strict, weak)
	}
	strict, weak, err = joined.Bounds(2)
	astTestMust(t, err)
	// The native vector join copies the right-hand bound indices unchanged.
	if len(strict) != 0 || !reflect.DeepEqual(weak, []uint32{1}) {
		t.Fatal("right joined bounds", strict, weak)
	}
	joined, err = JoinBoundRelations(left, right, []uint32{0}, []uint32{1})
	astTestMust(t, err)
	if joined.find(0) != joined.find(3) {
		t.Fatal("join columns were not equated")
	}
	if _, err = JoinBoundRelations(left, right, []uint32{2}, []uint32{0}); err == nil {
		t.Fatal("invalid join column accepted")
	}
	empty, err := JoinBoundRelations(NewBoundRelation(2, true), right, nil, nil)
	astTestMust(t, err)
	if !empty.Empty() {
		t.Fatal("empty join")
	}

	source := NewBoundRelation(3, false)
	astTestMust(t, source.AddLess(0, 2))
	astTestMust(t, source.AddLessEqual(0, 1))
	projected, err := source.Project([]uint32{1})
	astTestMust(t, err)
	strict, weak, err = projected.Bounds(0)
	astTestMust(t, err)
	if !reflect.DeepEqual(strict, []uint32{1}) || len(weak) != 0 {
		t.Fatal("projected bounds", strict, weak)
	}
	if _, err = source.Project([]uint32{2, 1}); err == nil {
		t.Fatal("unsorted projection accepted")
	}
	empty, err = NewBoundRelation(3, true).Project([]uint32{0})
	astTestMust(t, err)
	if !empty.Empty() {
		t.Fatal("empty projection")
	}
}

func TestBoundRelationProjectionPreservesEquivalence(t *testing.T) {
	r := NewBoundRelation(4, false)
	astTestMust(t, r.Equate(0, 1))
	astTestMust(t, r.Equate(2, 3))
	projected, err := r.Project([]uint32{0})
	astTestMust(t, err)
	if projected.find(1) != projected.find(2) || projected.find(0) == projected.find(1) {
		t.Fatal("projection changed equivalence classes")
	}
}
