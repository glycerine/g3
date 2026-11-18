package bv

import (
	"testing"
)

func TestBVBoundsModularRejectsExtendedDomain(t *testing.T) {
	malformed := bvBoundsSet{
		width: 8,
		ranges: []bvBoundsRange{
			{lo: 1, hi: 2},
			{lo: 4, hi: 5},
			{lo: 7, hi: 8},
		},
		tight: true,
	}
	if _, ok := malformed.modularInterval(); ok {
		t.Fatal("three disjoint ranges are outside the retained modular interval domain")
	}
	if _, ok := malformed.singleton(); ok || !malformed.complement().empty() {
		t.Fatal("malformed consumer input must fail closed")
	}
	if got := bvBoundsRangeSet(8, 0, 1).intersect(bvBoundsRangeSet(16, 0, 1)); !got.empty() {
		t.Fatalf("width-mismatched intersection = %#v", got)
	}
}
