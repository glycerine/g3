package realclosure

// Source: src/math/realclosure/realclosure.cpp neg, mul, div, div_rem and
// rem over value_ref_buffer, for the rational coefficients of the retained
// algebraic extensions. Coefficients are qm() rationals (s.numbers); every
// returned polynomial is owned by the caller and released with qPolyDel.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func newQPoly(n int) []numeral.Rational {
	out := make([]numeral.Rational, n)
	for i := range out {
		out[i] = numeral.NewRational()
	}
	return out
}

// qPolyDel releases every coefficient of p.
func (s *realValueStore) qPolyDel(p []numeral.Rational) (err error) {
	for i := len(p) - 1; i >= 0; i-- {
		err = errors.Join(err, s.numbers.Del(&p[i]))
	}
	return err
}

// qPolyTrim drops (and releases) trailing zero coefficients, the
// value_ref_buffer adjust_size of the source.
func (s *realValueStore) qPolyTrim(p []numeral.Rational) ([]numeral.Rational, error) {
	for len(p) > 0 {
		zero, err := s.numbers.IsZero(&p[len(p)-1])
		if err != nil {
			return p, err
		}
		if !zero {
			break
		}
		if err = s.numbers.Del(&p[len(p)-1]); err != nil {
			return p, err
		}
		p = p[:len(p)-1]
	}
	return p, nil
}

func (s *realValueStore) qPolyCopy(a []numeral.Rational) ([]numeral.Rational, error) {
	out := newQPoly(len(a))
	for i := range a {
		if err := s.numbers.Copy(&out[i], &a[i]); err != nil {
			return nil, errors.Join(err, s.qPolyDel(out))
		}
	}
	return out, nil
}

// qPolyNeg is neg(Rem.size(), Rem.c_ptr(), Q).
func (s *realValueStore) qPolyNeg(a []numeral.Rational) ([]numeral.Rational, error) {
	out, err := s.qPolyCopy(a)
	if err != nil {
		return nil, err
	}
	for i := range out {
		if err = s.numbers.Neg(&out[i]); err != nil {
			return nil, errors.Join(err, s.qPolyDel(out))
		}
	}
	return s.qPolyTrim(out)
}

// qPolyMul is mul(sz1, p1, sz2, p2, r).
func (s *realValueStore) qPolyMul(a, b []numeral.Rational) (out []numeral.Rational, err error) {
	if len(a) == 0 || len(b) == 0 {
		return nil, nil
	}
	out = newQPoly(len(a) + len(b) - 1)
	product := numeral.NewRational()
	defer func() { err = errors.Join(err, s.numbers.Del(&product)) }()
	for i := range a {
		for j := range b {
			if err = s.numbers.Mul(&a[i], &b[j], &product); err == nil {
				err = s.numbers.Add(&out[i+j], &product, &out[i+j])
			}
			if err != nil {
				return nil, errors.Join(err, s.qPolyDel(out))
			}
		}
	}
	return s.qPolyTrim(out)
}

// qPolyDiv is div(sz, p, b, r): every coefficient divided by b.
func (s *realValueStore) qPolyDiv(a []numeral.Rational, b *numeral.Rational) ([]numeral.Rational, error) {
	out := newQPoly(len(a))
	for i := range a {
		if err := s.numbers.Div(&a[i], b, &out[i]); err != nil {
			return nil, errors.Join(err, s.qPolyDel(out))
		}
	}
	return s.qPolyTrim(out)
}

// qPolyDivRem is div_rem(sz1, p1, sz2, p2, q, r) for a nonzero divisor.
func (s *realValueStore) qPolyDivRem(a, b []numeral.Rational) (quotient, remainder []numeral.Rational, ok bool, err error) {
	if len(b) == 0 {
		return nil, nil, false, nil
	}
	if remainder, err = s.qPolyCopy(a); err != nil {
		return nil, nil, false, err
	}
	if len(a) >= len(b) {
		quotient = newQPoly(len(a) - len(b) + 1)
	}
	ratio, product := numeral.NewRational(), numeral.NewRational()
	defer func() {
		err = errors.Join(err, s.numbers.Del(&ratio), s.numbers.Del(&product))
		if err != nil {
			err = errors.Join(err, s.qPolyDel(quotient), s.qPolyDel(remainder))
			quotient, remainder, ok = nil, nil, false
		}
	}()
	for len(remainder) >= len(b) {
		shift := len(remainder) - len(b)
		if err = s.numbers.Div(&remainder[len(remainder)-1], &b[len(b)-1], &ratio); err != nil {
			return
		}
		if err = s.numbers.Add(&quotient[shift], &ratio, &quotient[shift]); err != nil {
			return
		}
		for i := range b {
			if err = s.numbers.Mul(&ratio, &b[i], &product); err != nil {
				return
			}
			if err = s.numbers.Sub(&remainder[i+shift], &product, &remainder[i+shift]); err != nil {
				return
			}
		}
		if remainder, err = s.qPolyTrim(remainder); err != nil {
			return
		}
	}
	if quotient, err = s.qPolyTrim(quotient); err != nil {
		return
	}
	return quotient, remainder, true, nil
}

// qPolyRem is rem(sz1, p1, sz2, p2, r).
func (s *realValueStore) qPolyRem(a, modulus []numeral.Rational) ([]numeral.Rational, bool, error) {
	quotient, remainder, ok, err := s.qPolyDivRem(a, modulus)
	if err != nil {
		return nil, false, err
	}
	if err = s.qPolyDel(quotient); err != nil {
		return nil, false, errors.Join(err, s.qPolyDel(remainder))
	}
	return remainder, ok, nil
}
