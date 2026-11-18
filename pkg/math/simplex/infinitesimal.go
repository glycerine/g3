package simplex

// Source: src/util/inf_eps_rational.h (inf_eps_rational<inf_rational>,
// opt's inf_eps): c*infinity + a + b*epsilon over rationals, ordered
// lexicographically. The simplex solver's own eps_numeral is mpq_inf (the
// Main and Epsilon components; see solverValue).
//
// An Infinitesimal's rationals belong to the RationalManager its methods
// are given; the holder frees them with Del. A Go copy of the struct shares
// the rationals: copy values with Set or Clone.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

type Infinitesimal struct{ Infinity, Main, Epsilon numeral.Rational }

// ZeroInfinitesimal returns a valid inf_eps 0 (it owns nothing).
func ZeroInfinitesimal() Infinitesimal {
	return Infinitesimal{Infinity: numeral.NewRational(), Main: numeral.NewRational(), Epsilon: numeral.NewRational()}
}

// NewInfinitesimal is inf_eps(main + epsilon*eps) on q; a nil component is 0.
func NewInfinitesimal(q *numeral.RationalManager, main, epsilon *numeral.Rational) (Infinitesimal, error) {
	return NewInfiniteInfinitesimal(q, nil, main, epsilon)
}

// NewInfiniteInfinitesimal is inf_eps(infinity, inf_rational(main, epsilon))
// on q; a nil component is 0.
func NewInfiniteInfinitesimal(q *numeral.RationalManager, infinity, main, epsilon *numeral.Rational) (Infinitesimal, error) {
	r := ZeroInfinitesimal()
	var err error
	if infinity != nil {
		err = q.Copy(&r.Infinity, infinity)
	}
	if err == nil && main != nil {
		err = q.Copy(&r.Main, main)
	}
	if err == nil && epsilon != nil {
		err = q.Copy(&r.Epsilon, epsilon)
	}
	if err != nil {
		return r, errors.Join(err, r.Del(q))
	}
	return r, nil
}

// RationalInfinitesimal is inf_eps(v) on q.
func RationalInfinitesimal(q *numeral.RationalManager, v *numeral.Rational) (Infinitesimal, error) {
	return NewInfiniteInfinitesimal(q, nil, v, nil)
}

// InfiniteInfinitesimal is inf_eps(rational(sign), inf_rational(0)): oo for
// sign 1, -oo for sign -1.
func InfiniteInfinitesimal(q *numeral.RationalManager, sign int64) (Infinitesimal, error) {
	r := ZeroInfinitesimal()
	if err := q.SetInt64(&r.Infinity, sign); err != nil {
		return r, errors.Join(err, r.Del(q))
	}
	return r, nil
}

// Del frees a's rationals (the destructor) and leaves it a valid 0.
func (a *Infinitesimal) Del(q *numeral.RationalManager) error {
	err := errors.Join(q.Del(&a.Epsilon), q.Del(&a.Main), q.Del(&a.Infinity))
	*a = ZeroInfinitesimal()
	return err
}

// Set is the assignment a = b.
func (a *Infinitesimal) Set(q *numeral.RationalManager, b *Infinitesimal) error {
	if a == b {
		return nil
	}
	if err := q.Copy(&a.Infinity, &b.Infinity); err != nil {
		return err
	}
	if err := q.Copy(&a.Main, &b.Main); err != nil {
		return err
	}
	return q.Copy(&a.Epsilon, &b.Epsilon)
}

// CopyFrom sets a (on q) to b (on source).
func (a *Infinitesimal) CopyFrom(q *numeral.RationalManager, source *numeral.RationalManager, b *Infinitesimal) error {
	if err := q.CopyFrom(&a.Infinity, source, &b.Infinity); err != nil {
		return err
	}
	if err := q.CopyFrom(&a.Main, source, &b.Main); err != nil {
		return err
	}
	return q.CopyFrom(&a.Epsilon, source, &b.Epsilon)
}

// Clone returns a fresh copy of a (the copy constructor).
func (a *Infinitesimal) Clone(q *numeral.RationalManager) (Infinitesimal, error) {
	r := ZeroInfinitesimal()
	if err := r.Set(q, a); err != nil {
		return r, errors.Join(err, r.Del(q))
	}
	return r, nil
}

// Add is a += b.
func (a *Infinitesimal) Add(q *numeral.RationalManager, b *Infinitesimal) error {
	if err := q.Add(&a.Infinity, &b.Infinity, &a.Infinity); err != nil {
		return err
	}
	if err := q.Add(&a.Main, &b.Main, &a.Main); err != nil {
		return err
	}
	return q.Add(&a.Epsilon, &b.Epsilon, &a.Epsilon)
}

// Sub is a -= b.
func (a *Infinitesimal) Sub(q *numeral.RationalManager, b *Infinitesimal) error {
	if err := q.Sub(&a.Infinity, &b.Infinity, &a.Infinity); err != nil {
		return err
	}
	if err := q.Sub(&a.Main, &b.Main, &a.Main); err != nil {
		return err
	}
	return q.Sub(&a.Epsilon, &b.Epsilon, &a.Epsilon)
}

// Neg is a.neg().
func (a *Infinitesimal) Neg(q *numeral.RationalManager) error {
	if err := q.Neg(&a.Infinity); err != nil {
		return err
	}
	if err := q.Neg(&a.Main); err != nil {
		return err
	}
	return q.Neg(&a.Epsilon)
}

// Mul is a *= r.
func (a *Infinitesimal) Mul(q *numeral.RationalManager, r *numeral.Rational) error {
	if err := q.Mul(&a.Infinity, r, &a.Infinity); err != nil {
		return err
	}
	if err := q.Mul(&a.Main, r, &a.Main); err != nil {
		return err
	}
	return q.Mul(&a.Epsilon, r, &a.Epsilon)
}

// Quo is a /= r.
func (a *Infinitesimal) Quo(q *numeral.RationalManager, r *numeral.Rational) error {
	if err := q.Div(&a.Infinity, r, &a.Infinity); err != nil {
		return err
	}
	if err := q.Div(&a.Main, r, &a.Main); err != nil {
		return err
	}
	return q.Div(&a.Epsilon, r, &a.Epsilon)
}

// Sign is the sign (-1, 0 or 1) of the lexicographic value.
func (a *Infinitesimal) Sign(q *numeral.RationalManager) (int, error) {
	for _, r := range []*numeral.Rational{&a.Infinity, &a.Main, &a.Epsilon} {
		s, err := q.Sign(r)
		if err != nil || s != 0 {
			return normalSignOf(s), err
		}
	}
	return 0, nil
}

// Cmp compares a and b lexicographically.
func (a *Infinitesimal) Cmp(q *numeral.RationalManager, b *Infinitesimal) (int, error) {
	if c, err := q.Compare(&a.Infinity, &b.Infinity); err != nil || c != 0 {
		return c, err
	}
	if c, err := q.Compare(&a.Main, &b.Main); err != nil || c != 0 {
		return c, err
	}
	return q.Compare(&a.Epsilon, &b.Epsilon)
}

// IsZero reports a == 0.
func (a *Infinitesimal) IsZero(q *numeral.RationalManager) (bool, error) {
	s, err := a.Sign(q)
	if err != nil || s != 0 {
		return false, err
	}
	return true, nil
}

// IsRational is is_rational(): no infinity and no epsilon component.
func (a *Infinitesimal) IsRational(q *numeral.RationalManager) (bool, error) {
	z, err := q.IsZero(&a.Infinity)
	if err != nil || !z {
		return false, err
	}
	return q.IsZero(&a.Epsilon)
}

// Abs makes a nonnegative.
func (a *Infinitesimal) Abs(q *numeral.RationalManager) error {
	s, err := a.Sign(q)
	if err != nil || s >= 0 {
		return err
	}
	return a.Neg(q)
}

// String prints main[+eps*epsilon], prefixed by inf*infinity+ when infinite.
func (a *Infinitesimal) String(q *numeral.RationalManager) (string, error) {
	text, err := q.String(&a.Main)
	if err != nil {
		return "", err
	}
	if s, err := q.Sign(&a.Epsilon); err != nil {
		return "", err
	} else if s != 0 {
		eps, err := q.String(&a.Epsilon)
		if err != nil {
			return "", err
		}
		text += "+" + eps + "*epsilon"
	}
	if s, err := q.Sign(&a.Infinity); err != nil {
		return "", err
	} else if s != 0 {
		inf, err := q.String(&a.Infinity)
		if err != nil {
			return "", err
		}
		text = inf + "*infinity+" + text
	}
	return text, nil
}
