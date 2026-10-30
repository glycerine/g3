package bv

import (
	"math/big"
	"testing"
)

func TestBVBoundsModularBridgeWrappedRoundTrip(t *testing.T) {
	b := bvBoundsSet{
		width: 8,
		ranges: []bvBoundsRange{
			{lo: big.NewInt(0), hi: big.NewInt(15)},
			{lo: big.NewInt(240), hi: big.NewInt(255)},
		},
		tight: true,
	}
	v, ok := b.modularInterval()
	if !ok || !v.IsWrapped() || v.Lower().Int64() != 240 || v.Upper().Int64() != 15 || !v.Tight() {
		t.Fatalf("unexpected modular bridge: ok=%v value=%v", ok, v)
	}
	if got := bvBoundsFromModularInterval(v); !got.equal(b) {
		t.Fatalf("round trip mismatch: %#v", got)
	}
}
