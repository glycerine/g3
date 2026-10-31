package rel

import (
	"math/big"
	"testing"
)

func TestBoundRelationEqualityPreservesWeakOrderAndRejectsStrictOrder(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		for _, strict := range []bool{false, true} {
			r := NewBoundRelation(2, false)
			var err error
			if strict {
				err = r.AddLess(0, 1)
			} else {
				err = r.AddLessEqual(0, 1)
			}
			astTestMust(t, err)
			if reverse {
				err = r.Equate(1, 0)
			} else {
				err = r.Equate(0, 1)
			}
			astTestMust(t, err)
			if r.Empty() != strict {
				t.Fatalf("reverse=%t strict=%t: empty=%t", reverse, strict, r.Empty())
			}
			if !strict {
				lt, le, err := r.Bounds(0)
				astTestMust(t, err)
				if len(lt) != 0 || len(le) != 0 {
					t.Fatalf("redundant self bound: %v %v", lt, le)
				}
			}
		}
	}
}

func TestIntervalBoundProductIdenticalColumnsWithWeakOrder(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		p := NewIntervalBoundProduct(2, false)
		astTestMust(t, p.bound.AddLessEqual(0, 1))
		if reverse {
			astTestMust(t, p.FilterIdentical(1, 0))
		} else {
			astTestMust(t, p.FilterIdentical(0, 1))
		}
		if p.Empty() {
			t.Fatalf("weak order with equality became empty, reverse=%t", reverse)
		}
		if !p.ContainsFact([]*big.Rat{big.NewRat(2, 1), big.NewRat(2, 1)}) {
			t.Fatalf("equal fact disappeared, reverse=%t", reverse)
		}
	}
}

func TestBoundRelationStrictCycleThroughMergedColumn(t *testing.T) {
	r := NewBoundRelation(3, false)
	astTestMust(t, r.AddLess(0, 1))
	astTestMust(t, r.Equate(1, 2))
	if r.Empty() {
		t.Fatal("unrelated equality made strict order empty")
	}
	astTestMust(t, r.AddLessEqual(2, 0))
	if !r.Empty() {
		t.Fatal("strict cycle through an equality class was not detected")
	}
}
