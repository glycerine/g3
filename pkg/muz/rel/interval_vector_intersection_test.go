package rel

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
)

func intervalIntersectionBound(t *testing.T, relation *IntervalRelation, column uint32, lower, upper int64) {
	t.Helper()
	lo, loOpen, hi, hiOpen, err := relation.Bounds(column)
	if err != nil || loOpen || hiOpen || lo == nil || hi == nil || lo.Cmp(big.NewRat(lower, 1)) != 0 || hi.Cmp(big.NewRat(upper, 1)) != 0 {
		t.Fatalf("column %d bounds=(%v,%v,%v,%v) err=%v, want [%d,%d]", column, lo, loOpen, hi, hiOpen, err, lower, upper)
	}
}

func TestIntervalVectorIntersectionFiltersThroughJoinProjectSwap(t *testing.T) {
	target := NewIntervalFull(2)
	if err := target.FilterEqual(0, big.NewRat(5, 1)); err != nil {
		t.Fatal(err)
	}
	source := NewIntervalFull(2)
	if err := source.values.intersect(0, intervalClosed(3, 7)); err != nil {
		t.Fatal(err)
	}
	if err := source.values.intersect(1, intervalClosed(1, 9)); err != nil {
		t.Fatal(err)
	}
	if err := target.FilterByIntersection(source, []uint32{0}, []uint32{0}); err != nil {
		t.Fatal(err)
	}
	intervalIntersectionBound(t, target, 0, 5, 5)
	lo, _, hi, _, err := target.Bounds(1)
	if err != nil || lo != nil || hi != nil {
		t.Fatalf("unjoined target column changed: lo=%v hi=%v err=%v", lo, hi, err)
	}
}

func TestIntervalVectorIntersectionUsesSourceJoinEquivalenceSemantics(t *testing.T) {
	target := NewIntervalFull(2)
	source := NewIntervalFull(2)
	if err := source.FilterIdentical(0, 1); err != nil {
		t.Fatal(err)
	}
	if err := source.values.intersect(0, intervalClosed(2, 6)); err != nil {
		t.Fatal(err)
	}
	if err := target.FilterByIntersection(source, []uint32{0, 1}, []uint32{0, 1}); err != nil {
		t.Fatal(err)
	}
	// vector_relation::mk_join copies values through the source
	// representatives but does not recreate the source's internal equality
	// classes. Both target intervals are constrained independently.
	if target.values.find(0) == target.values.find(1) {
		t.Fatal("source equality unexpectedly survived the vector join")
	}
	intervalIntersectionBound(t, target, 0, 2, 6)
	intervalIntersectionBound(t, target, 1, 2, 6)

	empty := NewIntervalEmpty(2)
	if err := target.FilterByIntersection(empty, nil, nil); err != nil {
		t.Fatal(err)
	}
	if target.Empty() {
		t.Fatal("C++ intersection join/project returns full relation after empty join")
	}
	for column := uint32(0); column < 2; column++ {
		lower, _, upper, _, err := target.Bounds(column)
		if err != nil || lower != nil || upper != nil {
			t.Fatalf("empty-source intersection result column %d: lower=%v upper=%v err=%v", column, lower, upper, err)
		}
	}
}

func TestIntervalVectorIntersectionFailureAtomicity(t *testing.T) {
	target := NewIntervalFull(1)
	if err := target.FilterEqual(0, big.NewRat(4, 1)); err != nil {
		t.Fatal(err)
	}
	if err := target.FilterByIntersection(NewIntervalFull(1), []uint32{1}, []uint32{0}); !errors.Is(err, ast.ErrASTDomain) {
		t.Fatalf("invalid intersection error=%v", err)
	}
	intervalIntersectionBound(t, target, 0, 4, 4)
}

func intervalClosed(lower, upper int64) smtcore.RelationInterval {
	return smtcore.NewRelationInterval(big.NewRat(lower, 1), false, big.NewRat(upper, 1), false)
}
