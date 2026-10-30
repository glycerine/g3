package bv

import (
	"math/big"
	"testing"
)

func TestBVBoundsSourceRejectsNumeralBoundsWiderThanUint64(t *testing.T) {
	f := newBVBoundsFixture(t)
	sort, err := f.m.BitVectorSort(80)
	sharingMust(t, err)
	name, err := f.m.Symbols.Intern("bounds-wide-x")
	sharingMust(t, err)
	x, err := f.m.MkConst(name, sort)
	sharingMust(t, err)
	f.pin(t, x)
	value, err := f.m.BitVectorNumeralRat(big.NewRat(9, 1), 80)
	sharingMust(t, err)
	f.pin(t, value)
	bound, err := f.m.MkBitVectorULE(x, value)
	sharingMust(t, err)
	f.pin(t, bound)

	if _, _, found, err := f.simp.recognize(bound); err != nil || found {
		t.Fatalf("source is_number accepted an 80-bit bound: found=%t err=%v", found, err)
	}
	accepted, err := f.simp.AssertExpression(bound, false)
	sharingMust(t, err)
	if !accepted || len(f.simp.bounds) != 0 {
		t.Fatalf("wide constraint changed source bound state: accepted=%t bounds=%v", accepted, f.simp.bounds)
	}
	if simplify, err := f.simp.MaySimplify(bound); err != nil || simplify {
		t.Fatalf("wide bound entered the source simplification path: %t, %v", simplify, err)
	}
}
