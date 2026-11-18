package opt

import (
	"fmt"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func testRatGCD(a, b int64) int64 {
	if a < 0 {
		a = -a
	}
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func testRatNorm(n, d int64) (int64, int64) {
	if d < 0 {
		n, d = -n, -d
	}
	if g := testRatGCD(n, d); g > 1 {
		n, d = n/g, d/g
	}
	if n == 0 {
		d = 1
	}
	return n, d
}

// testRat is the small rational n/d; small rationals carry no manager cell.
func testRat(n, d int64) numeral.Rational {
	n, d = testRatNorm(n, d)
	if int64(int32(n)) != n || int64(int32(d)) != d {
		panic("testRat: value is not small")
	}
	return numeral.Rational{Num: numeral.NewInt(int32(n)), Den: numeral.NewInt(int32(d))}
}

func testRatPtr(n, d int64) *numeral.Rational {
	r := testRat(n, d)
	return &r
}

// ratIs reports r == n/d for a small r.
func ratIs(r *numeral.Rational, n, d int64) bool {
	n, d = testRatNorm(n, d)
	return r.IsSmall() && int64(r.Num.Val) == n && int64(r.Den.Val) == d
}

func ratSign(r *numeral.Rational) int { return r.Sign() }

func ratString(r *numeral.Rational) string {
	if !r.IsSmall() {
		return "<big>"
	}
	if r.Den.Val == 1 {
		return fmt.Sprint(r.Num.Val)
	}
	return fmt.Sprintf("%d/%d", r.Num.Val, r.Den.Val)
}

func ratEq(a, b *numeral.Rational) bool {
	return a.IsSmall() && b.IsSmall() && a.Num.Val == b.Num.Val && a.Den.Val == b.Den.Val
}
