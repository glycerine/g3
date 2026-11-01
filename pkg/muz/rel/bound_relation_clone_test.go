package rel

import (
	"reflect"
	"testing"
)

func TestBoundRelationCloneUsesSourceVectorCopySemantics(t *testing.T) {
	source := NewBoundRelation(3, false)
	if err := source.AddLess(0, 2); err != nil {
		t.Fatal(err)
	}
	if err := source.Equate(0, 1); err != nil {
		t.Fatal(err)
	}
	clone := source.Clone()
	if clone == nil || clone == source || clone.Empty() {
		t.Fatal("nonempty clone state", clone)
	}
	if clone.find(0) != 0 || clone.find(1) != 1 || clone.find(2) != 2 {
		t.Fatal("source vector copy unexpectedly preserved equality classes", clone.parent)
	}
	for _, column := range []uint32{0, 1} {
		strict, weak, err := clone.Bounds(column)
		if err != nil || !reflect.DeepEqual(strict, []uint32{2}) || len(weak) != 0 {
			t.Fatal("clone did not copy the source representative payload", column, strict, weak, err)
		}
	}
	if err := clone.AddLess(2, 0); err != nil {
		t.Fatal(err)
	}
	if !clone.Empty() {
		t.Fatal("clone mutation did not create a strict cycle")
	}
	if source.Empty() {
		t.Fatal("clone mutation changed the source")
	}
	strict, _, err := source.Bounds(0)
	if err != nil || !reflect.DeepEqual(strict, []uint32{2}) {
		t.Fatal("clone mutation changed source bounds", strict, err)
	}
}

func TestBoundRelationClonePreservesEmptyState(t *testing.T) {
	clone := NewBoundRelation(2, true).Clone()
	if clone == nil || !clone.Empty() || len(clone.columns) != 2 {
		t.Fatal("empty relation clone", clone)
	}
	for i := range clone.columns {
		if clone.parent[i] != uint32(i) || len(clone.columns[i].lt) != 0 || len(clone.columns[i].le) != 0 {
			t.Fatal("empty clone data", i, clone.parent, clone.columns[i])
		}
	}
}
