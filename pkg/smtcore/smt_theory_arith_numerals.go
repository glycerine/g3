package smtcore

// Source: src/util/inf_rational.h operations used by src/smt/theory_arith*.h.
// An smtArithmeticValue is inf_numeral: infinity + rational + infinitesimal
// in lexicographic order. mi_ext keeps infinity zero; i_ext keeps both
// infinity and infinitesimal zero. Never assign these structs by value:
// big.Rat shares backing storage, so use set.

import (
	"math"
	"math/big"
)

func (v *smtArithmeticValue) reset() {
	v.infinity.SetInt64(0)
	v.rational.SetInt64(0)
	v.infinitesimal.SetInt64(0)
}

func (v *smtArithmeticValue) setRational(q *big.Rat) {
	v.infinity.SetInt64(0)
	v.rational.Set(q)
	v.infinitesimal.SetInt64(0)
}

func (v *smtArithmeticValue) neg() {
	v.infinity.Neg(&v.infinity)
	v.rational.Neg(&v.rational)
	v.infinitesimal.Neg(&v.infinitesimal)
}

func (v *smtArithmeticValue) mulRat(q *big.Rat) {
	smtRatMul(&v.infinity, &v.infinity, q)
	smtRatMul(&v.rational, &v.rational, q)
	smtRatMul(&v.infinitesimal, &v.infinitesimal, q)
}

func (v *smtArithmeticValue) quoRat(q *big.Rat) {
	smtRatQuo(&v.infinity, &v.infinity, q)
	smtRatQuo(&v.rational, &v.rational, q)
	smtRatQuo(&v.infinitesimal, &v.infinitesimal, q)
}

// addMul sets v += q*x; subMul sets v -= q*x.
func (v *smtArithmeticValue) addMul(q *big.Rat, x *smtArithmeticValue) {
	var t big.Rat
	smtRatAdd(&v.infinity, &v.infinity, smtRatMul(&t, q, &x.infinity))
	smtRatAdd(&v.rational, &v.rational, smtRatMul(&t, q, &x.rational))
	smtRatAdd(&v.infinitesimal, &v.infinitesimal, smtRatMul(&t, q, &x.infinitesimal))
}

func (v *smtArithmeticValue) subMul(q *big.Rat, x *smtArithmeticValue) {
	var t big.Rat
	smtRatSub(&v.infinity, &v.infinity, smtRatMul(&t, q, &x.infinity))
	smtRatSub(&v.rational, &v.rational, smtRatMul(&t, q, &x.rational))
	smtRatSub(&v.infinitesimal, &v.infinitesimal, smtRatMul(&t, q, &x.infinitesimal))
}

func (v *smtArithmeticValue) isZero() bool {
	return v.infinity.Sign() == 0 && v.rational.Sign() == 0 && v.infinitesimal.Sign() == 0
}

// sign is the sign of the lexicographically leading nonzero component.
func (v *smtArithmeticValue) sign() int {
	if s := v.infinity.Sign(); s != 0 {
		return s
	}
	if s := v.rational.Sign(); s != 0 {
		return s
	}
	return v.infinitesimal.Sign()
}

func (v *smtArithmeticValue) isRational() bool {
	return v.infinity.Sign() == 0 && v.infinitesimal.Sign() == 0
}

// isInt is inf_rational::is_int.
func (v *smtArithmeticValue) isInt() bool {
	return v.isRational() && v.rational.IsInt()
}

func (v *smtArithmeticValue) equal(other *smtArithmeticValue) bool { return v.cmp(other) == 0 }

// isOne / isMinusOne compare the whole inf_numeral with 1 / -1.
func (v *smtArithmeticValue) isOne() bool {
	return v.isRational() && smtRatIsOne(&v.rational)
}
func (v *smtArithmeticValue) isMinusOne() bool {
	return v.isRational() && smtRatIsMinusOne(&v.rational)
}
func (v *smtArithmeticValue) abs() {
	if v.sign() < 0 {
		v.neg()
	}
}

// smtInfFloor is floor(inf_rational): the largest integer not above the
// value, with the infinitesimal deciding at integral rational parts.
func smtInfFloor(v *smtArithmeticValue) *big.Rat {
	if v.rational.IsInt() {
		r := new(big.Rat).Set(&v.rational)
		if v.infinitesimal.Sign() < 0 {
			r.Sub(r, big.NewRat(1, 1))
		}
		return r
	}
	return new(big.Rat).SetInt(smtRatFloor(&v.rational))
}

// smtInfCeil is ceil(inf_rational).
func smtInfCeil(v *smtArithmeticValue) *big.Rat {
	if v.rational.IsInt() {
		r := new(big.Rat).Set(&v.rational)
		if v.infinitesimal.Sign() > 0 {
			r.Add(r, big.NewRat(1, 1))
		}
		return r
	}
	r := new(big.Rat).SetInt(smtRatFloor(&v.rational))
	return r.Add(r, big.NewRat(1, 1))
}

func smtRatIsOne(q *big.Rat) bool { return q.IsInt() && q.Num().IsInt64() && q.Num().Int64() == 1 }
func smtRatIsMinusOne(q *big.Rat) bool {
	return q.IsInt() && q.Num().IsInt64() && q.Num().Int64() == -1
}

// smtRatIsBig is rational::is_big: the numerator or denominator does not fit
// the small (machine int) representation.
func smtRatIsBig(q *big.Rat) bool {
	small := func(x *big.Int) bool {
		return x.IsInt64() && x.Int64() >= math.MinInt32 && x.Int64() <= math.MaxInt32
	}
	return !small(q.Num()) || !small(q.Denom())
}
