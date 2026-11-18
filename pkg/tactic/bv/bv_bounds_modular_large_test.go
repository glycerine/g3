package bv

import (
	"math/big"
	"testing"
)

// bv_bounds_tactic.cpp's interval holds uint64_t bounds with sz <= 64, so
// an 80-bit bound set has no modular interval.
func TestBVBoundsModularLargeWidthConsumer(t *testing.T) {
	maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 80), big.NewInt(1))
	lower := new(big.Int).Sub(maximum, big.NewInt(31))
	b := bvBoundsSet{
		width: 80,
		ranges: []bvBoundsRange{
			{lo: big.NewInt(0), hi: big.NewInt(15)},
			{lo: lower, hi: maximum},
		},
		tight: true,
	}
	if v, ok := b.modularInterval(); ok {
		t.Fatalf("80-bit modular interval accepted: %v", v)
	}
}
