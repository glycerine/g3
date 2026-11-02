package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestBoundVectorIntersectionPropagatesSourceOrder(t *testing.T) {
	target := NewBoundRelation(2, false)
	source := NewBoundRelation(2, false)
	if err := source.AddLess(0, 1); err != nil {
		t.Fatal(err)
	}
	if err := target.FilterByIntersection(source, []uint32{0, 1}, []uint32{0, 1}); err != nil {
		t.Fatal(err)
	}
	strict, weak, err := target.Bounds(0)
	if err != nil || len(strict) != 1 || strict[0] != 1 || len(weak) != 0 {
		t.Fatalf("propagated bounds strict=%v weak=%v err=%v", strict, weak, err)
	}
	if target.Empty() {
		t.Fatal("consistent source order emptied target")
	}
}

func TestBoundVectorIntersectionEmptyAliasAndFailureAtomicity(t *testing.T) {
	target := NewBoundRelation(2, false)
	if err := target.AddLessEqual(0, 1); err != nil {
		t.Fatal(err)
	}
	if err := target.FilterByIntersection(NewBoundRelation(2, true), nil, nil); err != nil {
		t.Fatal(err)
	}
	if !target.Empty() {
		t.Fatal("intersection with empty source did not clear target")
	}

	alias := NewBoundRelation(2, false)
	if err := alias.AddLess(0, 1); err != nil {
		t.Fatal(err)
	}
	if err := alias.FilterByIntersection(alias, []uint32{0, 1}, []uint32{0, 1}); err != nil {
		t.Fatal(err)
	}
	strict, _, err := alias.Bounds(0)
	if err != nil || len(strict) != 1 || strict[0] != 1 {
		t.Fatalf("aliased intersection bounds=%v err=%v", strict, err)
	}

	stable := NewBoundRelation(2, false)
	if err := stable.AddLess(0, 1); err != nil {
		t.Fatal(err)
	}
	if err := stable.FilterByIntersection(NewBoundRelation(2, false), []uint32{2}, []uint32{0}); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatalf("invalid intersection error=%v", err)
	}
	strict, _, err = stable.Bounds(0)
	if err != nil || len(strict) != 1 || strict[0] != 1 || stable.Empty() {
		t.Fatalf("failed intersection mutated target: strict=%v empty=%v err=%v", strict, stable.Empty(), err)
	}
}
