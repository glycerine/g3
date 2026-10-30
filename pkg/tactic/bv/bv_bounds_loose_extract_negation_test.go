package bv

import (
	"math/big"
	"testing"
)

func TestBVBoundsNegatedExtractEqualityTargetsExtractTerm(t *testing.T) {
	f := newBVBoundsFixture(t)
	top, err := f.m.MkBitVectorExtract(7, 4, f.x)
	sharingMust(t, err)
	zero, err := f.m.BitVectorNumeralUnsigned(0, 4)
	sharingMust(t, err)
	eq, err := f.m.MkEq(zero, top)
	sharingMust(t, err)

	key, recognized, found, err := f.simp.recognize(eq)
	sharingMust(t, err)
	if !found || key != top || recognized.width != 4 || !recognized.tight {
		t.Fatalf("extract-zero recognition = found %v, key %v, bound %#v", found, key, recognized)
	}
	if ok, err := f.simp.AssertExpression(eq, true); err != nil || !ok {
		t.Fatalf("assert negated extract-zero equality: ok=%v err=%v", ok, err)
	}
	got := f.simp.bounds[top]
	if !got.equal(bvBoundsRangeSet(4, big.NewInt(1), big.NewInt(15))) {
		t.Fatalf("negated equality must complement the extract term's singleton bound: %#v", got)
	}
}
