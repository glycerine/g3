package bv

import (
	"testing"
)

func TestBVBoundsSourceEqualityBoundTargetsExtractTerm(t *testing.T) {
	f := newBVBoundsFixture(t)
	top, e := f.m.MkBitVectorExtract(7, 4, f.x)
	sharingMust(t, e)
	zero4, e := f.m.BitVectorNumeralUnsigned(0, 4)
	sharingMust(t, e)
	eq, e := f.m.MkEq(zero4, top)
	sharingMust(t, e)
	key, b, found, e := f.simp.recognize(eq)
	sharingMust(t, e)
	zero, singleton := b.singleton()
	if !found || key != top || b.width != 4 || !singleton || zero.Sign() != 0 {
		t.Fatal(found, key, b)
	}
}
