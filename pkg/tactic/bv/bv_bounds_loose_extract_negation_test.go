package bv

import "testing"

func TestBVBoundsLooseExtractNegationWidensToFull(t *testing.T) {
	f := newBVBoundsFixture(t)
	top, err := f.m.MkBitVectorExtract(7, 4, f.x)
	sharingMust(t, err)
	zero, err := f.m.BitVectorNumeralUnsigned(0, 4)
	sharingMust(t, err)
	eq, err := f.m.MkEq(zero, top)
	sharingMust(t, err)

	key, recognized, found, err := f.simp.recognize(eq)
	sharingMust(t, err)
	if !found || key != f.x || recognized.tight {
		t.Fatalf("extract-zero recognition = found %v, bound %#v", found, recognized)
	}
	if ok, err := f.simp.AssertExpression(eq, true); err != nil || !ok {
		t.Fatalf("assert negated extract-zero equality: ok=%v err=%v", ok, err)
	}
	got := f.simp.bounds[f.x]
	if !got.full() || !got.tight {
		t.Fatalf("non-tight complement must widen to tight full interval: %#v", got)
	}
}
