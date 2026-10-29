package bv

import (
	"math/big"
	"testing"
)

func TestBVBoundsSourceModularSetIntersectionComplement(t *testing.T) {
	a := bvBoundsRangeSet(8, big.NewInt(4), big.NewInt(20))
	b := bvBoundsRangeSet(8, big.NewInt(10), big.NewInt(30))
	got := a.intersect(b)
	want := bvBoundsRangeSet(8, big.NewInt(10), big.NewInt(20))
	if !got.equal(want) {
		t.Fatal(got)
	}
	if !got.implies(a) || a.implies(got) {
		t.Fatal("implication")
	}
	if !a.intersect(a.complement()).empty() {
		t.Fatal("complement overlap")
	}
}
