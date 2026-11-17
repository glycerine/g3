package smtcore

// Source: src/util/inf_rational.h, src/util/inf_eps_rational.h and
// src/util/rational.h operations used by src/smt/theory_arith*.h.
//
// numeral is z3's rational: a numeral.Rational whose operations run on the
// AST manager's synchronized mpq manager (rational's g_mpq_manager). An
// smtArithmeticValue is inf_numeral: infinity + rational + infinitesimal in
// lexicographic order. mi_ext keeps infinity zero (inf_rational); i_ext
// keeps infinity and infinitesimal zero (rational). A zero infinity takes
// no mpq operation, as inf_rational has no such component.
//
// A numeral.Rational may own a cell: never copy one by value (use set /
// Copy), and free it with del where the source's destructor or scoped
// numeral frees it. The zero Go value is not a number (its denominator is
// 0): initialize with smtQZero / init before use.

import (
	"math"
	"math/big"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// smtQZero is rational(0); smtQInt is rational(int).
func smtQZero() numeral.Rational { return numeral.NewRational() }
func smtQInt(v int32) numeral.Rational {
	return numeral.Rational{Num: numeral.NewInt(v), Den: numeral.NewInt(1)}
}

// smtQSetInt sets r = v.
func smtQSetInt(q *numeral.RationalManager, r *numeral.Rational, v int64) error {
	return q.SetInt64(r, v)
}

// smtQFromBig sets r to a math/big value (a boundary conversion).
func smtQFromBig(q *numeral.RationalManager, r *numeral.Rational, x *big.Rat) error {
	return q.SetBigRat(r, x)
}

// smtQBig returns r as a new math/big value (a boundary conversion).
func smtQBig(q *numeral.RationalManager, r *numeral.Rational) (*big.Rat, error) {
	out := new(big.Rat)
	return out, q.BigRatOf(r, out)
}

// smtQNew returns a fresh copy of r (rational's copy constructor).
func smtQNew(q *numeral.RationalManager, r *numeral.Rational) (numeral.Rational, error) {
	out := smtQZero()
	return out, q.Copy(&out, r)
}

// smtQDel is rational's destructor: mpz's del frees only a cell, so an
// inline value takes no manager call.
func smtQDel(q *numeral.RationalManager, r *numeral.Rational) error {
	if r.IsSmall() {
		return nil
	}
	return q.Del(r)
}

// smtQDelAll frees a vector of rationals and empties it.
func smtQDelAll(q *numeral.RationalManager, rs []numeral.Rational) ([]numeral.Rational, error) {
	var err error
	for i := range rs {
		if e := smtQDel(q, &rs[i]); e != nil && err == nil {
			err = e
		}
	}
	return rs[:0], err
}

// smtQFloor is floor(r); smtQCeil is ceil(r).
func smtQFloor(q *numeral.RationalManager, r, out *numeral.Rational) error { return q.Floor(r, out) }
func smtQCeil(q *numeral.RationalManager, r, out *numeral.Rational) error  { return q.Ceil(r, out) }

// smtQIsBig is rational::is_big.
func smtQIsBig(r *numeral.Rational) bool { return !r.IsSmall() }

// smtQStorageSize is rational::storage_size: mpz_manager::size_info of the
// numerator plus that of the denominator (1 for an inline mpz, m_size + 1
// for a cell).
func smtQStorageSize(q *numeral.RationalManager, r *numeral.Rational) (uint, error) {
	if r.IsSmall() {
		return 2, nil
	}
	n, err := q.StorageSize(r)
	return uint(n), err
}

// smtQZeroShortcut: in a release build an infinitesimal operation whose
// result is its unchanged zero or nonzero operand (0 + x, x * 0, 0 = 0 ...)
// takes no mpq call. The value and its representation are those the call
// leaves; g3trace and g3debug builds make every call, so each mpq trace and
// assertion is kept.
var smtQZeroShortcut = !diagnostic.TraceCompiled() && !diagnostic.DebugBuild

func newSMTArithmeticValue() smtArithmeticValue {
	return smtArithmeticValue{infinity: smtQZero(), rational: smtQZero(), infinitesimal: smtQZero()}
}

// init makes a value that owns no cell zero.
func (v *smtArithmeticValue) init() { *v = newSMTArithmeticValue() }

func (v *smtArithmeticValue) del(q *numeral.RationalManager) error {
	err := smtQDel(q, &v.infinity)
	if e := smtQDel(q, &v.rational); e != nil && err == nil {
		err = e
	}
	if e := smtQDel(q, &v.infinitesimal); e != nil && err == nil {
		err = e
	}
	return err
}

// smtArithValuesDel frees a vector of values and empties it.
func smtArithValuesDel(q *numeral.RationalManager, vs []smtArithmeticValue) ([]smtArithmeticValue, error) {
	var err error
	for i := range vs {
		if e := vs[i].del(q); e != nil && err == nil {
			err = e
		}
	}
	return vs[:0], err
}

func (v *smtArithmeticValue) reset(q *numeral.RationalManager) error {
	if !v.infinity.IsZero() {
		if err := q.Reset(&v.infinity); err != nil {
			return err
		}
	}
	if err := q.Reset(&v.rational); err != nil {
		return err
	}
	if smtQZeroShortcut && v.infinitesimal.IsZero() && v.infinitesimal.IsSmall() && v.infinitesimal.Den.Val == 1 {
		return nil
	}
	return q.Reset(&v.infinitesimal)
}

func (v *smtArithmeticValue) set(q *numeral.RationalManager, other *smtArithmeticValue) error {
	if v == other {
		return nil
	}
	if !v.infinity.IsZero() || !other.infinity.IsZero() {
		if err := q.Copy(&v.infinity, &other.infinity); err != nil {
			return err
		}
	}
	if err := q.Copy(&v.rational, &other.rational); err != nil {
		return err
	}
	if smtQZeroShortcut && v.infinitesimal.IsZero() && other.infinitesimal.IsZero() && v.infinitesimal.IsSmall() && v.infinitesimal.Den.Val == 1 {
		return nil
	}
	return q.Copy(&v.infinitesimal, &other.infinitesimal)
}

// setRational is inf_numeral = rational.
func (v *smtArithmeticValue) setRational(q *numeral.RationalManager, r *numeral.Rational) error {
	if !v.infinity.IsZero() {
		if err := q.Reset(&v.infinity); err != nil {
			return err
		}
	}
	if err := q.Copy(&v.rational, r); err != nil {
		return err
	}
	return q.Reset(&v.infinitesimal)
}

// setInt is inf_numeral = rational(k).
func (v *smtArithmeticValue) setInt(q *numeral.RationalManager, k int64) error {
	if !v.infinity.IsZero() {
		if err := q.Reset(&v.infinity); err != nil {
			return err
		}
	}
	if err := q.SetInt64(&v.rational, k); err != nil {
		return err
	}
	return q.Reset(&v.infinitesimal)
}

func (v *smtArithmeticValue) neg(q *numeral.RationalManager) error {
	if !v.infinity.IsZero() {
		if err := q.Neg(&v.infinity); err != nil {
			return err
		}
	}
	if err := q.Neg(&v.rational); err != nil {
		return err
	}
	if smtQZeroShortcut && v.infinitesimal.IsZero() {
		return nil
	}
	return q.Neg(&v.infinitesimal)
}

func (v *smtArithmeticValue) add(q *numeral.RationalManager, other *smtArithmeticValue) error {
	if !other.infinity.IsZero() {
		if err := q.Add(&v.infinity, &other.infinity, &v.infinity); err != nil {
			return err
		}
	}
	if err := q.Add(&v.rational, &other.rational, &v.rational); err != nil {
		return err
	}
	if smtQZeroShortcut && other.infinitesimal.IsZero() {
		return nil
	}
	return q.Add(&v.infinitesimal, &other.infinitesimal, &v.infinitesimal)
}

func (v *smtArithmeticValue) sub(q *numeral.RationalManager, other *smtArithmeticValue) error {
	if !other.infinity.IsZero() {
		if err := q.Sub(&v.infinity, &other.infinity, &v.infinity); err != nil {
			return err
		}
	}
	if err := q.Sub(&v.rational, &other.rational, &v.rational); err != nil {
		return err
	}
	if smtQZeroShortcut && other.infinitesimal.IsZero() {
		return nil
	}
	return q.Sub(&v.infinitesimal, &other.infinitesimal, &v.infinitesimal)
}

// mulRat is inf_numeral *= rational; quoRat is inf_numeral /= rational.
func (v *smtArithmeticValue) mulRat(q *numeral.RationalManager, r *numeral.Rational) error {
	if !v.infinity.IsZero() {
		if err := q.Mul(&v.infinity, r, &v.infinity); err != nil {
			return err
		}
	}
	if err := q.Mul(&v.rational, r, &v.rational); err != nil {
		return err
	}
	if smtQZeroShortcut && v.infinitesimal.IsZero() {
		return nil
	}
	return q.Mul(&v.infinitesimal, r, &v.infinitesimal)
}

func (v *smtArithmeticValue) quoRat(q *numeral.RationalManager, r *numeral.Rational) error {
	if !v.infinity.IsZero() {
		if err := q.Div(&v.infinity, r, &v.infinity); err != nil {
			return err
		}
	}
	if err := q.Div(&v.rational, r, &v.rational); err != nil {
		return err
	}
	if smtQZeroShortcut && v.infinitesimal.IsZero() {
		return nil // the rational division already rejected r = 0
	}
	return q.Div(&v.infinitesimal, r, &v.infinitesimal)
}

// addMul sets v += r*x; subMul sets v -= r*x.
func (v *smtArithmeticValue) addMul(q *numeral.RationalManager, r *numeral.Rational, x *smtArithmeticValue) error {
	if !x.infinity.IsZero() {
		if err := q.AddMul(&v.infinity, r, &x.infinity, &v.infinity); err != nil {
			return err
		}
	}
	if err := q.AddMul(&v.rational, r, &x.rational, &v.rational); err != nil {
		return err
	}
	if smtQZeroShortcut && x.infinitesimal.IsZero() {
		return nil
	}
	return q.AddMul(&v.infinitesimal, r, &x.infinitesimal, &v.infinitesimal)
}

func (v *smtArithmeticValue) subMul(q *numeral.RationalManager, r *numeral.Rational, x *smtArithmeticValue) error {
	if !x.infinity.IsZero() {
		if err := q.SubMul(&v.infinity, r, &x.infinity, &v.infinity); err != nil {
			return err
		}
	}
	if err := q.SubMul(&v.rational, r, &x.rational, &v.rational); err != nil {
		return err
	}
	if smtQZeroShortcut && x.infinitesimal.IsZero() {
		return nil
	}
	return q.SubMul(&v.infinitesimal, r, &x.infinitesimal, &v.infinitesimal)
}

func (v *smtArithmeticValue) cmp(q *numeral.RationalManager, other *smtArithmeticValue) (int, error) {
	if !v.infinity.IsZero() || !other.infinity.IsZero() {
		if c, err := q.Compare(&v.infinity, &other.infinity); err != nil || c != 0 {
			return c, err
		}
	}
	if c, err := q.Compare(&v.rational, &other.rational); err != nil || c != 0 {
		return c, err
	}
	if smtQZeroShortcut && v.infinitesimal.IsZero() && other.infinitesimal.IsZero() {
		return 0, nil
	}
	return q.Compare(&v.infinitesimal, &other.infinitesimal)
}

func (v *smtArithmeticValue) lt(q *numeral.RationalManager, other *smtArithmeticValue) (bool, error) {
	c, err := v.cmp(q, other)
	return c < 0, err
}

func (v *smtArithmeticValue) equal(q *numeral.RationalManager, other *smtArithmeticValue) (bool, error) {
	c, err := v.cmp(q, other)
	return c == 0, err
}

func (v *smtArithmeticValue) isZero() bool {
	return v.infinity.IsZero() && v.rational.IsZero() && v.infinitesimal.IsZero()
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
	return v.infinity.IsZero() && v.infinitesimal.IsZero()
}

// isInt is inf_rational::is_int.
func (v *smtArithmeticValue) isInt() bool {
	return v.isRational() && v.rational.IsInt()
}

// isOne / isMinusOne compare the whole inf_numeral with 1 / -1.
func (v *smtArithmeticValue) isOne() bool      { return v.isRational() && v.rational.IsOne() }
func (v *smtArithmeticValue) isMinusOne() bool { return v.isRational() && v.rational.IsMinusOne() }

func (v *smtArithmeticValue) abs(q *numeral.RationalManager) error {
	if v.sign() < 0 {
		return v.neg(q)
	}
	return nil
}

// smtInfFloor is floor(inf_rational): the largest integer not above the
// value, with the infinitesimal deciding at integral rational parts.
func smtInfFloor(q *numeral.RationalManager, v *smtArithmeticValue, out *numeral.Rational) error {
	if v.rational.IsInt() {
		if err := q.Copy(out, &v.rational); err != nil {
			return err
		}
		if v.infinitesimal.IsNeg() {
			return q.Dec(out)
		}
		return nil
	}
	return q.Floor(&v.rational, out)
}

// smtInfCeil is ceil(inf_rational).
func smtInfCeil(q *numeral.RationalManager, v *smtArithmeticValue, out *numeral.Rational) error {
	if v.rational.IsInt() {
		if err := q.Copy(out, &v.rational); err != nil {
			return err
		}
		if v.infinitesimal.IsPos() {
			return q.Inc(out)
		}
		return nil
	}
	return q.Ceil(&v.rational, out)
}

// smtQIntPtr is a fresh rational(v), for a constant argument.
func smtQIntPtr(v int32) *numeral.Rational {
	r := smtQInt(v)
	return &r
}

// smtQMapKey is a Go map key for the source's hash tables keyed by
// rational: equal values have equal keys. A value whose numerator and
// denominator fit an int is kept in n/d (big empty), any other value as
// big.Rat.RatString text. It is read without a trace.
type smtQMapKey struct {
	n, d int32
	big  string
}

func smtQKey(q *numeral.RationalManager, r *numeral.Rational) (smtQMapKey, error) {
	if r.IsSmall() {
		return smtQMapKey{n: r.Num.Val, d: r.Den.Val}, nil
	}
	b, err := smtQBig(q, r)
	if err != nil {
		return smtQMapKey{}, err
	}
	if n, d := b.Num(), b.Denom(); n.IsInt64() && d.IsInt64() {
		if nv, dv := n.Int64(), d.Int64(); nv >= math.MinInt32 && nv <= math.MaxInt32 && dv <= math.MaxInt32 {
			return smtQMapKey{n: int32(nv), d: int32(dv)}, nil
		}
	}
	return smtQMapKey{big: b.RatString()}, nil
}

// qkey is smtQKey with failures kept in qErr.
func (t *SMTTheoryArith) qkey(r *numeral.Rational) smtQMapKey {
	k, err := smtQKey(t.q, r)
	t.keep(err)
	return k
}
