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
