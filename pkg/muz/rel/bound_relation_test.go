package rel_test

import (
	"math/big"
	"reflect"
	"testing"

	"github.com/glycerine/g3/pkg/muz/rel"
)

func TestBoundRelationOrderPropagationAndCycles(t *testing.T) {
	r := rel.NewBoundRelation(4, false)
	if err := r.AddLessEqual(1, 2); err != nil {
		t.Fatal(err)
	}
	if err := r.AddLess(2, 3); err != nil {
		t.Fatal(err)
	}
	if err := r.AddLessEqual(1, 2); err != nil {
		t.Fatal(err)
	}
	if err := r.AddLess(0, 1); err != nil {
		t.Fatal(err)
	}
	strict, weak, err := r.Bounds(0)
	if err != nil || !reflect.DeepEqual(strict, []uint32{1, 2, 3}) || len(weak) != 0 {
		t.Fatal("strict propagation", strict, weak, err)
	}
	strict, weak, err = r.Bounds(1)
	if err != nil || !reflect.DeepEqual(strict, []uint32{3}) || !reflect.DeepEqual(weak, []uint32{2}) {
		t.Fatal("weak propagation", strict, weak, err)
	}
	if err = r.AddLessEqual(3, 0); err != nil {
		t.Fatal(err)
	}
	if !r.Empty() {
		t.Fatal("strict cycle must be empty")
	}
	weakCycle := rel.NewBoundRelation(2, false)
	if err = weakCycle.AddLessEqual(0, 1); err != nil {
		t.Fatal(err)
	}
	if err = weakCycle.AddLessEqual(1, 0); err != nil {
		t.Fatal(err)
	}
	if weakCycle.Empty() {
		t.Fatal("weak cycle is satisfiable")
	}
	if err = weakCycle.AddLessEqual(0, 1); err != nil {
		t.Fatal(err)
	}
	if err = weakCycle.AddLess(0, 1); err != nil {
		t.Fatal(err)
	}
	if !weakCycle.Empty() {
		t.Fatal("strict edge in weak cycle must be empty")
	}
}

func TestBoundRelationUnionInterval(t *testing.T) {
	n := func(v int64) *big.Rat { return big.NewRat(v, 1) }
	interval := rel.NewIntervalEmpty(3)
	if err := interval.AddFact([]*big.Rat{n(1), n(2), n(2)}); err != nil {
		t.Fatal(err)
	}
	r := rel.NewBoundRelation(3, false)
	if err := r.AddLess(0, 1); err != nil {
		t.Fatal(err)
	}
	if err := r.AddLess(1, 2); err != nil {
		t.Fatal(err)
	}
	if err := r.AddLessEqual(1, 2); err != nil {
		t.Fatal(err)
	}
	if err := r.AddLess(0, 1); err != nil {
		t.Fatal(err)
	}
	if err := r.UnionInterval(interval); err != nil {
		t.Fatal(err)
	}
	strict, _, err := r.Bounds(0)
	if err != nil || !reflect.DeepEqual(strict, []uint32{1, 2}) {
		t.Fatal("strict interval bounds", strict, err)
	}
	strict, weak, err := r.Bounds(1)
	if err != nil || len(strict) != 0 || !reflect.DeepEqual(weak, []uint32{2}) {
		t.Fatal("equal endpoint handling", strict, weak, err)
	}
	if err = r.UnionInterval(rel.NewIntervalFull(3)); err != nil {
		t.Fatal(err)
	}
	strict, weak, err = r.Bounds(0)
	if err != nil || len(strict) != 0 || len(weak) != 0 {
		t.Fatal("infinite interval must clear bounds", strict, weak, err)
	}
}
