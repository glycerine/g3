package api

import "testing"

// AUDIT.md api_01-F7: Z3_mk_bvsdiv_no_overflow makes (= t1 min) before
// the -1 numeral; Z3_mk_bv2int(signed) makes the 2^sz bound before zero and
// bvslt.
func TestBVSDivNoOverflowConstructionOrder(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	sort := f.pin(f.c.BitVectorSort(8))
	a := f.pin(f.c.Constant("order-a", sort))
	b := f.pin(f.c.Constant("order-b", sort))
	f.pin(f.c.BitVectorSDivNoOverflow(a, b))
	min := f.pin(f.c.BitVectorSignedMin(sort))
	eq := f.pin(f.c.Equal(a, min))
	minusOne := f.pin(f.c.BitVectorValue(-1, sort))
	eqID, err := f.c.ASTID(eq)
	astTestMust(t, err)
	oneID, err := f.c.ASTID(minusOne)
	astTestMust(t, err)
	if eqID >= oneID {
		t.Fatalf("(= a min) id %d not below -1 id %d", eqID, oneID)
	}
}

func TestBVToIntSignedConstructionOrder(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	sort := f.pin(f.c.BitVectorSort(8))
	a := f.pin(f.c.Constant("order-a", sort))
	f.pin(f.c.BitVectorToInt(a, true))
	integer := f.pin(f.c.IntegerSort())
	bound := f.pin(f.c.Numeral("256", integer))
	zero := f.pin(f.c.BitVectorValue(0, sort))
	slt := f.pin(f.c.BitVectorSLT(a, zero))
	boundID, err := f.c.ASTID(bound)
	astTestMust(t, err)
	sltID, err := f.c.ASTID(slt)
	astTestMust(t, err)
	if boundID >= sltID {
		t.Fatalf("2^8 id %d not below bvslt id %d", boundID, sltID)
	}
}
