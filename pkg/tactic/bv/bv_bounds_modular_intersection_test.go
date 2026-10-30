package bv

import (
	"math/big"
	"testing"
)

func TestBVBoundsModularIntersectionAndImplication(t *testing.T) {
	a := bvBoundsRangeSet(8, big.NewInt(4), big.NewInt(20))
	b := bvBoundsRangeSet(8, big.NewInt(10), big.NewInt(30))
	got := a.intersect(b)
	want := bvBoundsRangeSet(8, big.NewInt(10), big.NewInt(20))
	if !got.equal(want) || !got.tight {
		t.Fatalf("intersection = %#v", got)
	}
	if !got.implies(a) || !got.implies(b) || a.implies(got) {
		t.Fatal("source interval implication relation was not retained")
	}

	loose := bvBoundsLooseRangeSet(8, big.NewInt(12), big.NewInt(18))
	got = a.intersect(loose)
	if got.tight || len(got.ranges) != 1 || got.ranges[0].lo.Int64() != 12 || got.ranges[0].hi.Int64() != 18 {
		t.Fatalf("loose intersection = %#v", got)
	}
}
