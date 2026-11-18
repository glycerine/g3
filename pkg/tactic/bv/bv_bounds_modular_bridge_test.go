package bv

import (
	"testing"
)

func TestBVBoundsModularBridgeWrappedRoundTrip(t *testing.T) {
	b := bvBoundsSet{
		width: 8,
		ranges: []bvBoundsRange{
			{lo: 0, hi: 15},
			{lo: 240, hi: 255},
		},
		tight: true,
	}
	v, ok := b.modularInterval()
	if !ok || !v.IsWrapped() || v.Lower() != 240 || v.Upper() != 15 || !v.Tight() {
		t.Fatalf("unexpected modular bridge: ok=%v value=%v", ok, v)
	}
	if got := bvBoundsFromModularInterval(v); !got.equal(b) {
		t.Fatalf("round trip mismatch: %#v", got)
	}
}
