package bv

import "testing"

func TestBVBoundsSourceContradictionAndNegatedBound(t *testing.T) {
	f := newBVBoundsFixture(t)
	ok, e := f.simp.AssertExpression(f.ule(t, f.x, f.num(t, 10)), false)
	sharingMust(t, e)
	if !ok {
		t.Fatal()
	}
	ok, e = f.simp.AssertExpression(f.ule(t, f.num(t, 20), f.x), false)
	sharingMust(t, e)
	if ok {
		t.Fatal("disjoint bounds accepted")
	}
	sharingMust(t, f.simp.Pop(1))
	ok, e = f.simp.AssertExpression(f.ule(t, f.x, f.num(t, 10)), true)
	sharingMust(t, e)
	if !ok {
		t.Fatal("complement bound")
	}
	if len(f.simp.bounds) != 1 {
		t.Fatal(f.simp.bounds)
	}
	for _, b := range f.simp.bounds {
		if len(b.ranges) != 1 || b.ranges[0].lo.Int64() != 11 {
			t.Fatal(b)
		}
	}
}
