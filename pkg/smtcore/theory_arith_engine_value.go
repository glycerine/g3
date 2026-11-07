package smtcore

// Source: src/util/inf_rational.h operations used by src/smt/theory_arith*.h.
// An smtArithmeticValue is inf_numeral: infinity + rational + infinitesimal
// in lexicographic order. mi_ext keeps infinity zero; i_ext keeps both
// infinity and infinitesimal zero. Never assign these structs by value:
// big.Rat shares backing storage, so use set.

import "math/big"

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
	v.infinity.Mul(&v.infinity, q)
	v.rational.Mul(&v.rational, q)
	v.infinitesimal.Mul(&v.infinitesimal, q)
}

func (v *smtArithmeticValue) quoRat(q *big.Rat) {
	v.infinity.Quo(&v.infinity, q)
	v.rational.Quo(&v.rational, q)
	v.infinitesimal.Quo(&v.infinitesimal, q)
}

// addMul sets v += q*x; subMul sets v -= q*x.
func (v *smtArithmeticValue) addMul(q *big.Rat, x *smtArithmeticValue) {
	var t big.Rat
	v.infinity.Add(&v.infinity, t.Mul(q, &x.infinity))
	v.rational.Add(&v.rational, t.Mul(q, &x.rational))
	v.infinitesimal.Add(&v.infinitesimal, t.Mul(q, &x.infinitesimal))
}

func (v *smtArithmeticValue) subMul(q *big.Rat, x *smtArithmeticValue) {
	var t big.Rat
	v.infinity.Sub(&v.infinity, t.Mul(q, &x.infinity))
	v.rational.Sub(&v.rational, t.Mul(q, &x.rational))
	v.infinitesimal.Sub(&v.infinitesimal, t.Mul(q, &x.infinitesimal))
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
