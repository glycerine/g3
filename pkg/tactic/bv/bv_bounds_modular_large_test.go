package bv

import (
	"testing"
)

// bv_bounds_tactic.cpp's interval holds uint64_t bounds with sz <= 64, so
// an 80-bit bound set has no modular interval.
func TestBVBoundsModularLargeWidthConsumer(t *testing.T) {
	maximum := ^uint64(0)
	lower := maximum - 31
	b := bvBoundsSet{
		width: 80,
		ranges: []bvBoundsRange{
			{lo: 0, hi: 15},
			{lo: lower, hi: maximum},
		},
		tight: true,
	}
	if v, ok := b.modularInterval(); ok {
		t.Fatalf("80-bit modular interval accepted: %v", v)
	}
}
