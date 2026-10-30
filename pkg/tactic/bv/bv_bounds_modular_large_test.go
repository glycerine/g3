package bv

import (
	"math/big"
	"testing"
)

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
	v, ok := b.modularInterval()
	if !ok || !v.IsWrapped() || v.Size() != 80 || v.Lower().Cmp(lower) != 0 || v.Upper().Cmp(big.NewInt(15)) != 0 {
		t.Fatalf("large modular bridge failed: ok=%v value=%v", ok, v)
	}
	negated := b.complement()
	if negated.tight || len(negated.ranges) != 1 || negated.ranges[0].lo.Cmp(big.NewInt(16)) != 0 || negated.ranges[0].hi.Cmp(new(big.Int).Sub(lower, big.NewInt(1))) != 0 {
		t.Fatalf("large complement = %#v", negated)
	}
}
