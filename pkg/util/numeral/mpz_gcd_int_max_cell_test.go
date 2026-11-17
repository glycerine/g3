package numeral

import (
	"math"
	"testing"
)

// TestIntegerGCDIntMaxCell mirrors the fork's tst_z3_4_7_1_gcd_int_max_cell:
// SetUint64 keeps the source's strict "< INT_MAX" inline rule, so INT_MAX is
// stored in a cell, while SetInt64 keeps it inline. The Lehmer loop needs a1
// in a cell whenever b1 is; the unpatched loop read the inline a1's cell.
func TestIntegerGCDIntMaxCell(t *testing.T) {
	m, _ := integerManager(t)
	var a, b, c Int
	if e := m.SetInt64(&a, math.MaxInt32); e != nil {
		t.Fatal(e)
	}
	if e := m.SetUint64(&b, math.MaxInt32); e != nil {
		t.Fatal(e)
	}
	if !a.IsSmall() || b.IsSmall() {
		t.Fatalf("expected inline a and cell b: %v %v", a.IsSmall(), b.IsSmall())
	}
	for _, order := range [][2]*Int{{&a, &b}, {&b, &a}} {
		if e := m.GCD(order[0], order[1], &c); e != nil {
			t.Fatal(e)
		}
		if got := integerString(t, m, &c); got != "2147483647" {
			t.Fatal(got)
		}
	}
	m.Del(&a)
	m.Del(&b)
	m.Del(&c)
	for _, synch := range []bool{false, true} {
		q, _ := rationalManager(t, synch)
		var r Rational
		if e := q.SetFraction(&r, math.MaxInt32, math.MaxInt32); e != nil {
			t.Fatal(e)
		}
		if one, e := q.IsOne(&r); e != nil || !one {
			t.Fatal("SetFraction(INT_MAX, INT_MAX) is not one", e)
		}
		q.Del(&r)
	}
}
