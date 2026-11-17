package numeral

import (
	"math"
	"math/big"
	"testing"
)

// TestRationalValuePredicatesAndBigRat checks the manager-free predicates
// against the manager's, and SetBigRat/BigRatOf round trips, on inline,
// int32-edge and cell values, for both manager modes.
func TestRationalValuePredicatesAndBigRat(t *testing.T) {
	two64 := new(big.Int).Lsh(big.NewInt(1), 64)
	values := []*big.Rat{
		big.NewRat(0, 1), big.NewRat(1, 1), big.NewRat(-1, 1), big.NewRat(7, 3), big.NewRat(-7, 3),
		big.NewRat(math.MaxInt32, 1), big.NewRat(math.MinInt32, 1), big.NewRat(1, math.MaxInt32),
		big.NewRat(math.MaxInt32+1, 1), big.NewRat(math.MinInt32-1, 1),
		new(big.Rat).SetInt(two64), new(big.Rat).SetFrac(big.NewInt(-3), two64),
		new(big.Rat).SetFrac(new(big.Int).Neg(two64), big.NewInt(7)),
	}
	for _, synch := range []bool{false, true} {
		m, _ := rationalManager(t, synch)
		for _, x := range values {
			r := NewRational()
			if e := m.SetBigRat(&r, x); e != nil {
				t.Fatal(e)
			}
			var back big.Rat
			if e := m.BigRatOf(&r, &back); e != nil || back.Cmp(x) != 0 {
				t.Fatal("round trip", x, &back, e)
			}
			s, _ := m.Sign(&r)
			isInt, _ := m.IsInt(&r)
			one, _ := m.IsOne(&r)
			minusOne, _ := m.IsMinusOne(&r)
			zero, _ := m.IsZero(&r)
			if r.Sign() != int(s) && !(r.Sign() == 1 && s > 0) && !(r.Sign() == -1 && s < 0) || r.Sign() != x.Sign() ||
				r.IsInt() != isInt || r.IsOne() != one || r.IsMinusOne() != minusOne || r.IsZero() != zero ||
				r.IsPos() != (x.Sign() > 0) || r.IsNeg() != (x.Sign() < 0) {
				t.Fatal("predicates", x)
			}
			if v, ok := r.SmallInt32(); ok != (r.IsSmall() && x.IsInt()) || ok && int64(v) != x.Num().Int64() {
				t.Fatal("SmallInt32", x)
			}
			if want := x.Num().IsInt64() && x.Num().Int64() >= math.MinInt32 && x.Num().Int64() <= math.MaxInt32 && x.Denom().IsInt64() && x.Denom().Int64() <= math.MaxInt32; r.IsSmall() != want {
				t.Fatal("representation", x, r.IsSmall())
			}
			if e := m.Del(&r); e != nil {
				t.Fatal(e)
			}
		}
	}
}
