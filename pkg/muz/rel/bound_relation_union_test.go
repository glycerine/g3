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

func TestBoundRelationUnionDelta(t *testing.T) {
	target, source, delta := NewBoundRelation(2, true), NewBoundRelation(2, false), NewBoundRelation(2, true)
	astTestMust(t, source.AddLess(0, 1))
	changed, err := target.UnionWithDelta(source, delta, false)
	astTestMust(t, err)
	if !changed || delta.Empty() {
		t.Fatal("initial delta")
	}
	strict, _, err := delta.Bounds(0)
	astTestMust(t, err)
	if !reflect.DeepEqual(strict, []uint32{1}) {
		t.Fatal("initial delta bounds", strict)
	}
	changed, err = target.UnionWithDelta(source, delta, true)
	astTestMust(t, err)
	if changed {
		t.Fatal("unchanged widening")
	}
	changed, err = target.UnionWithDelta(NewBoundRelation(2, true), delta, false)
	astTestMust(t, err)
	if changed || !delta.Empty() {
		t.Fatal("empty source must reset delta")
	}
	if _, err = target.UnionWithDelta(source, target, false); err == nil {
		t.Fatal("aliased delta accepted")
	}
}

func TestBoundRelationUnionIntoEmptyCopiesEqualityClasses(t *testing.T) {
	source := NewBoundRelation(3, false)
	astTestMust(t, source.Equate(0, 1))
	astTestMust(t, source.AddLess(0, 2))

	target := NewBoundRelation(3, true)
	changed, err := target.Union(source)
	astTestMust(t, err)
	if !changed || target.Empty() || target.find(0) != target.find(1) {
		t.Fatal("union into empty lost source equality", changed, target.Empty())
	}
	strict, _, err := target.Bounds(1)
	astTestMust(t, err)
	if !reflect.DeepEqual(strict, []uint32{2}) {
		t.Fatal("union into empty lost bounds", strict)
	}

	deltaTarget := NewBoundRelation(3, true)
	delta := NewBoundRelation(3, true)
	changed, err = deltaTarget.UnionWithDelta(source, delta, false)
	astTestMust(t, err)
	if !changed || delta.find(0) != delta.find(1) {
		t.Fatal("delta copy lost source equality", changed)
	}
	strict, _, err = delta.Bounds(1)
	astTestMust(t, err)
	if !reflect.DeepEqual(strict, []uint32{2}) {
		t.Fatal("delta copy lost bounds", strict)
	}
}

func TestBoundRelationEmptySourceCopiesItsDeltaState(t *testing.T) {
	source := NewBoundRelation(2, false)
	astTestMust(t, source.Equate(0, 1))
	astTestMust(t, source.AddLess(0, 0))
	if !source.Empty() {
		t.Fatal("strict self-bound did not make source empty")
	}
	target, delta := NewBoundRelation(2, false), NewBoundRelation(2, false)
	changed, err := target.UnionWithDelta(source, delta, false)
	astTestMust(t, err)
	if changed || !delta.Empty() || delta.find(0) != delta.find(1) {
		t.Fatal("empty source delta did not copy source state", changed, delta.Empty())
	}
}
