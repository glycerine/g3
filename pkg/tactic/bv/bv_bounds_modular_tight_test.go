package bv

import (
	"testing"
)

func TestBVBoundsModularTightComplementPolicy(t *testing.T) {
	b := bvBoundsRangeSet(8, 4, 20)
	negated := b.complement()
	if negated.tight || len(negated.ranges) != 2 || negated.ranges[0].lo != 0 || int64(negated.ranges[0].hi) != 3 || int64(negated.ranges[1].lo) != 21 || int64(negated.ranges[1].hi) != 255 {
		t.Fatalf("tight complement = %#v", negated)
	}
	widened := negated.complement()
	if !widened.full() || !widened.tight {
		t.Fatalf("negating the source's loose complement must widen: %#v", widened)
	}
}
