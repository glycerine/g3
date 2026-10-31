package rel

import (
	"reflect"
	"testing"
)

func TestBoundRelationUnionAndWiden(t *testing.T) {
	left, right := NewBoundRelation(3, false), NewBoundRelation(3, false)
	astTestMust(t, left.AddLess(0, 2))
	astTestMust(t, left.AddLessEqual(0, 1))
	astTestMust(t, right.AddLess(0, 2))
	changed, err := left.Union(right)
	astTestMust(t, err)
	strict, weak, err := left.Bounds(0)
	astTestMust(t, err)
	if !changed || !reflect.DeepEqual(strict, []uint32{2}) || len(weak) != 0 {
		t.Fatal("shared bound union", changed, strict, weak)
	}
	changed, err = left.Widen(right)
	astTestMust(t, err)
	if changed {
		t.Fatal("unchanged widening")
	}
	empty := NewBoundRelation(3, true)
	changed, err = empty.Union(right)
	astTestMust(t, err)
	if !changed || empty.Empty() {
		t.Fatal("union into empty")
	}
	strict, _, err = empty.Bounds(0)
	astTestMust(t, err)
	if !reflect.DeepEqual(strict, []uint32{2}) {
		t.Fatal("union into empty lost bound", strict)
	}
	changed, err = left.Union(NewBoundRelation(3, true))
	astTestMust(t, err)
	if changed {
		t.Fatal("empty source changed union")
	}
}

func TestBoundRelationUnionSplitsEqualities(t *testing.T) {
	left, right := NewBoundRelation(3, false), NewBoundRelation(3, false)
	astTestMust(t, left.AddLess(0, 2))
	astTestMust(t, left.Equate(0, 1))
	astTestMust(t, right.AddLess(0, 2))
	astTestMust(t, right.AddLess(1, 2))
	changed, err := left.Union(right)
	astTestMust(t, err)
	if !changed || left.find(0) == left.find(1) {
		t.Fatal("joint equivalences were not split")
	}
	for _, col := range []uint32{0, 1} {
		strict, _, err := left.Bounds(col)
		astTestMust(t, err)
		if !reflect.DeepEqual(strict, []uint32{2}) {
			t.Fatal("lost shared bound", col, strict)
		}
	}
}
