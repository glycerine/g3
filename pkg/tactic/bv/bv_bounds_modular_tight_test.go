package bv

import (
	"math/big"
	"testing"
)

func TestBVBoundsModularTightComplementPolicy(t *testing.T) {
	b := bvBoundsRangeSet(8, big.NewInt(4), big.NewInt(20))
	negated := b.complement()
	if negated.tight || len(negated.ranges) != 2 || negated.ranges[0].lo.Sign() != 0 || negated.ranges[0].hi.Int64() != 3 || negated.ranges[1].lo.Int64() != 21 || negated.ranges[1].hi.Int64() != 255 {
		t.Fatalf("tight complement = %#v", negated)
	}
	widened := negated.complement()
	if !widened.full() || !widened.tight {
		t.Fatalf("negating the source's loose complement must widen: %#v", widened)
	}
}
