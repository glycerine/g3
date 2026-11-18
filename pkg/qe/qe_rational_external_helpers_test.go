package qe_test

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// smallRat is the small rational n/d (d > 0, gcd 1); it holds no numeral
// storage, so it needs no manager and no destruction.
func smallRat(n, d int64) *unumeral.Rational {
	r := unumeral.Rational{Num: unumeral.NewInt(int32(n)), Den: unumeral.NewInt(int32(d))}
	return &r
}

// isSmallRat reports whether r is the small rational n/d.
func isSmallRat(r *unumeral.Rational, n, d int64) bool {
	return r.IsSmall() && int64(r.Num.Val) == n && int64(r.Den.Val) == d
}
