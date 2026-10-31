package rel

// Source: src/test/dl_relation.cpp test_interval_relation.

import (
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestIntervalRelationSourceOperations(t *testing.T) {
	m := astTestManager(t)
	family, err := m.RegisterArithmeticPlugin()
	astTestMust(t, err)
	sort, err := m.MkFamilySort(family, 1, nil)
	astTestMust(t, err)
	own := func(h ast.ASTHandle, err error) ast.ASTHandle { astTestMust(t, err); astTestPin(t, m, h); return h }
	x0, x1 := own(m.MkVar(0, sort)), own(m.MkVar(1, sort))
	zero, one := own(m.ArithmeticIntegerNumeralText("0")), own(m.ArithmeticIntegerNumeralText("1"))
	empty, full := NewIntervalEmpty(4), NewIntervalFull(4)
	if !empty.Empty() || full.Empty() {
		t.Fatal("source empty/full initialization")
	}
	astTestMust(t, full.FilterInterpreted(m, own(m.MkArithmeticLE(x0, zero))))
	joined, err := JoinIntervalRelations(full, full, []uint32{1, 2}, []uint32{2, 3})
	astTestMust(t, err)
	astTestMust(t, full.FilterEqual(2, big.NewRat(4, 1)))
	astTestMust(t, full.FilterIdentical(1, 2))
	fact := func(a int64) []*big.Rat {
		return []*big.Rat{big.NewRat(a, 1), big.NewRat(4, 1), big.NewRat(4, 1), big.NewRat(5, 1)}
	}
	for _, tc := range []struct {
		a    int64
		want bool
	}{{0, true}, {-1, true}, {1, false}} {
		if got := full.ContainsFact(fact(tc.a)); got != tc.want {
			t.Fatal("interval fact", tc.a, got)
		}
	}
	renamed, err := full.Rename([]uint32{2, 3})
	astTestMust(t, err)
	if !renamed.ContainsFact([]*big.Rat{big.NewRat(0, 1), big.NewRat(4, 1), big.NewRat(5, 1), big.NewRat(4, 1)}) {
		t.Fatal("renamed interval")
	}
	astTestMust(t, full.FilterInterpreted(m, own(m.MkArithmeticLE(x1, one))))
	if !full.Empty() {
		t.Fatal("contradictory interpreted bound")
	}
	projected, err := joined.Project([]uint32{2, 3})
	astTestMust(t, err)
	if projected.Empty() {
		t.Fatal("projection of source join became empty")
	}
}

func TestBoundRelationSourceOperations(t *testing.T) {
	empty, full := NewBoundRelation(4, true), NewBoundRelation(4, false)
	if !empty.Empty() || full.Empty() {
		t.Fatal("source empty/full initialization")
	}
	joined, err := JoinBoundRelations(full, full, []uint32{1, 2}, []uint32{2, 3})
	astTestMust(t, err)
	astTestMust(t, full.AddLess(2, 3))
	astTestMust(t, full.Equate(1, 2))
	if full.find(1) != full.find(2) || full.Empty() {
		t.Fatal("source equality and strict bound")
	}
	renamed, err := full.Rename([]uint32{0, 2, 3})
	astTestMust(t, err)
	if renamed.Empty() {
		t.Fatal("renamed source relation")
	}
	projected, err := joined.Project([]uint32{2, 3})
	astTestMust(t, err)
	if projected.Empty() {
		t.Fatal("projected source join")
	}
	for _, which := range []int{0, 1} {
		left, right := NewBoundRelation(4, false), NewBoundRelation(4, false)
		astTestMust(t, left.Equate(1, 3))
		if which == 0 {
			astTestMust(t, left.AddLess(0, 1))
			astTestMust(t, left.AddLess(1, 2))
			astTestMust(t, right.Equate(2, 3))
			astTestMust(t, right.AddLess(0, 3))
		} else {
			astTestMust(t, left.Equate(2, 3))
			astTestMust(t, left.AddLess(0, 1))
			astTestMust(t, right.Equate(1, 3))
			astTestMust(t, right.AddLess(0, 2))
			astTestMust(t, right.AddLess(0, 3))
		}
		_, err := left.Union(right)
		astTestMust(t, err)
		if left.Empty() {
			t.Fatal("source union became empty", which)
		}
		strict, _, err := left.Bounds(0)
		astTestMust(t, err)
		found := false
		for _, column := range strict {
			found = found || left.find(column) == left.find(3)
		}
		if !found {
			t.Fatal("source union lost common bound x0 < x3", which, strict)
		}
	}
}
