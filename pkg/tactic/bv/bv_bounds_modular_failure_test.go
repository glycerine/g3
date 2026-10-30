package bv

import (
	"math/big"
	"testing"
)

func TestBVBoundsModularRejectsExtendedDomain(t *testing.T) {
	malformed := bvBoundsSet{
		width: 8,
		ranges: []bvBoundsRange{
			{lo: big.NewInt(1), hi: big.NewInt(2)},
			{lo: big.NewInt(4), hi: big.NewInt(5)},
			{lo: big.NewInt(7), hi: big.NewInt(8)},
		},
		tight: true,
	}
	if _, ok := malformed.modularInterval(); ok {
		t.Fatal("three disjoint ranges are outside the retained modular interval domain")
	}
	if _, ok := malformed.singleton(); ok || !malformed.complement().empty() {
		t.Fatal("malformed consumer input must fail closed")
	}
	if got := bvBoundsRangeSet(8, big.NewInt(0), big.NewInt(1)).intersect(bvBoundsRangeSet(16, big.NewInt(0), big.NewInt(1))); !got.empty() {
		t.Fatalf("width-mismatched intersection = %#v", got)
	}
}
