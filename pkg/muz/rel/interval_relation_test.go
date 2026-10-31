package rel_test

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/muz/rel"
)

func TestIntervalRelationFactAndJoin(t *testing.T) {
	n := func(v int64) *big.Rat { return big.NewRat(v, 1) }
	r := rel.NewIntervalEmpty(2)
	if !r.Empty() {
		t.Fatal("new empty relation")
	}
	if err := r.AddFact([]*big.Rat{n(1), n(2)}); err != nil {
		t.Fatal(err)
	}
	if err := r.AddFact([]*big.Rat{n(3), n(4)}); err != nil {
		t.Fatal(err)
	}
	if !r.ContainsFact([]*big.Rat{n(2), n(3)}) || r.ContainsFact([]*big.Rat{n(0), n(3)}) {
		t.Fatal("fact hull")
	}
	if err := r.FilterEqual(0, n(2)); err != nil {
		t.Fatal(err)
	}
	if err := r.FilterIdentical(0, 1); err != nil {
		t.Fatal(err)
	}
	if !r.ContainsFact([]*big.Rat{n(2), n(2)}) || r.ContainsFact([]*big.Rat{n(2), n(3)}) {
		t.Fatal("equal columns")
	}
	other := rel.NewIntervalEmpty(1)
	if err := other.AddFact([]*big.Rat{n(2)}); err != nil {
		t.Fatal(err)
	}
	joined, err := rel.JoinIntervalRelations(r, other, []uint32{0}, []uint32{0})
	if err != nil {
		t.Fatal(err)
	}
	if !joined.ContainsFact([]*big.Rat{n(2), n(2), n(2)}) {
		t.Fatal("joined fact")
	}
	projected, err := joined.Project([]uint32{0, 1})
	if err != nil || !projected.ContainsFact([]*big.Rat{n(2)}) {
		t.Fatalf("projected fact: %v", err)
	}
	lo, _, hi, _, err := projected.Bounds(0)
	if err != nil || lo.Cmp(n(2)) != 0 || hi.Cmp(n(2)) != 0 {
		t.Fatal("projected bounds")
	}
	lo.SetInt64(99)
	if !projected.ContainsFact([]*big.Rat{n(2)}) {
		t.Fatal("bounds escaped mutable state")
	}
	emptyProjection, err := rel.NewIntervalEmpty(2).Project([]uint32{0})
	if err != nil || !emptyProjection.Empty() {
		t.Fatal("projection of empty relation must remain empty")
	}
}

func TestIntervalRelationRename(t *testing.T) {
	n := func(v int64) *big.Rat { return big.NewRat(v, 1) }
	r := rel.NewIntervalEmpty(3)
	if err := r.AddFact([]*big.Rat{n(1), n(2), n(3)}); err != nil {
		t.Fatal(err)
	}
	renamed, err := r.Rename([]uint32{0, 1, 2})
	if err != nil || !renamed.ContainsFact([]*big.Rat{n(3), n(1), n(2)}) {
		t.Fatalf("cycle rename: %v", err)
	}
	if !r.ContainsFact([]*big.Rat{n(1), n(2), n(3)}) {
		t.Fatal("rename changed source")
	}
	if _, err := r.Rename([]uint32{0, 0}); err == nil {
		t.Fatal("duplicate cycle column accepted")
	}
	if _, err := r.Rename([]uint32{3}); err == nil {
		t.Fatal("out-of-range cycle column accepted")
	}
	empty, err := rel.NewIntervalEmpty(2).Rename([]uint32{0, 1})
	if err != nil || !empty.Empty() {
		t.Fatal("rename must preserve emptiness")
	}
}

func TestIntervalRelationUnionDelta(t *testing.T) {
	n := func(v int64) *big.Rat { return big.NewRat(v, 1) }
	target, source, delta := rel.NewIntervalEmpty(1), rel.NewIntervalEmpty(1), rel.NewIntervalEmpty(1)
	if err := source.AddFact([]*big.Rat{n(1)}); err != nil {
		t.Fatal(err)
	}
	changed, err := target.UnionWithDelta(source, delta, false)
	if err != nil || !changed || !delta.ContainsFact([]*big.Rat{n(1)}) {
		t.Fatalf("initial delta: %v", err)
	}
	if err := source.AddFact([]*big.Rat{n(2)}); err != nil {
		t.Fatal(err)
	}
	changed, err = target.UnionWithDelta(source, delta, false)
	if err != nil || !changed || !delta.ContainsFact([]*big.Rat{n(2)}) {
		t.Fatalf("changed delta: %v", err)
	}
	changed, err = target.UnionWithDelta(source, delta, false)
	if err != nil || changed || !delta.ContainsFact([]*big.Rat{n(1)}) {
		t.Fatalf("unchanged delta: %v", err)
	}
	changed, err = target.UnionWithDelta(rel.NewIntervalEmpty(1), delta, false)
	if err != nil || changed || !delta.Empty() {
		t.Fatalf("empty source delta: %v", err)
	}
	if _, err = target.UnionWithDelta(source, target, false); err == nil {
		t.Fatal("aliased delta accepted")
	}
}
