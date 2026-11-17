package numeral

// Source: src/util/mpq.h is_zero, is_pos, is_neg, sign, is_int, is_one,
// is_minus_one, and src/util/rational.h's inline predicates over them.
// mpq_manager reads these from the value alone (the numerator's sign, an
// inline denominator of one) without touching the manager, and so do these.
// A big numerator's Val is its sign, as mpz's m_val is.

import (
	"math"
	"math/big"
)

// IsZero is mpq_manager::is_zero.
func (a *Rational) IsZero() bool { return a.Num.Val == 0 && a.Num.Cell == 0 }

// Sign is mpq_manager::sign: -1, 0 or 1.
func (a *Rational) Sign() int {
	switch {
	case a.Num.Val > 0:
		return 1
	case a.Num.Val < 0:
		return -1
	}
	return 0
}

// IsPos and IsNeg are mpq_manager::is_pos and is_neg.
func (a *Rational) IsPos() bool { return a.Num.Val > 0 }
func (a *Rational) IsNeg() bool { return a.Num.Val < 0 }

// IsInt is mpq_manager::is_int: the denominator is one.
func (a *Rational) IsInt() bool { return isIntegerRational(a) }

// IsOne and IsMinusOne are mpq_manager::is_one and is_minus_one.
func (a *Rational) IsOne() bool { return isOneInteger(&a.Num) && isOneInteger(&a.Den) }
func (a *Rational) IsMinusOne() bool {
	return a.Num.IsSmall() && a.Num.Val == -1 && isOneInteger(&a.Den)
}

// SmallInt32 returns the value when it is an inline integer.
func (a *Rational) SmallInt32() (int32, bool) {
	if a.Num.Cell == 0 && isIntegerRational(a) {
		return a.Num.Val, true
	}
	return 0, false
}

// SetBigRat sets a to the reduced Go rational r. It is the conversion at the
// boundaries where g3 code outside the mpq port still holds math/big values;
// z3 copies a rational there, so it emits no trace. An integer numerator or
// denominator in the int range is stored inline (set(int64)); larger ones
// are set from their 32-bit digits, which mpz's set_digits normalizes.
func (m *RationalManager) SetBigRat(a *Rational, r *big.Rat) error {
	if e := m.check(a); e != nil {
		return e
	}
	num := r.Num()
	if num.IsInt64() {
		if n := num.Int64(); n >= math.MinInt32 && n <= math.MaxInt32 {
			if e := m.Z.SetInt64(&a.Num, n); e != nil {
				return e
			}
		} else if e := m.setBigInt(&a.Num, num); e != nil {
			return e
		}
	} else if e := m.setBigInt(&a.Num, num); e != nil {
		return e
	}
	if r.IsInt() {
		return m.Z.SetInt64(&a.Den, 1)
	}
	den := r.Denom()
	if den.IsInt64() && den.Int64() <= math.MaxInt32 {
		return m.Z.SetInt64(&a.Den, den.Int64())
	}
	return m.setBigInt(&a.Den, den)
}

func (m *RationalManager) setBigInt(a *Int, x *big.Int) error {
	if e := m.Z.SetLimbs(a, bigIntLimbs32(x)); e != nil {
		return e
	}
	if x.Sign() < 0 {
		return m.Z.Neg(a)
	}
	return nil
}

// BigRatOf sets out to a's value, read from its digits (no to_string, so no
// trace): the other direction of SetBigRat.
func (m *RationalManager) BigRatOf(a *Rational, out *big.Rat) error {
	if e := m.check(a); e != nil {
		return e
	}
	if a.Num.Cell == 0 && a.Den.Cell == 0 {
		if a.Den.Val == 1 {
			out.SetInt64(int64(a.Num.Val))
			return nil
		}
		out.SetFrac64(int64(a.Num.Val), int64(a.Den.Val))
		return nil
	}
	num, e := integerBigInt(m.Z, &a.Num)
	if e != nil {
		return e
	}
	den, e := integerBigInt(m.Z, &a.Den)
	if e != nil {
		return e
	}
	out.SetFrac(num, den)
	return nil
}
