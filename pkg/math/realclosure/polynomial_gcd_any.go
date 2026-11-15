package realclosure

// Source: src/math/realclosure/realclosure.cpp:3497-3525.

import "errors"

func (s *realValueStore) gcdAnyPolynomials(left, right []*realValue, usePrem bool) (result []*realValue, err error) {
	a, e := s.retainRationalPolynomial(left)
	if e != nil {
		return nil, e
	}
	b, e := s.retainRationalPolynomial(right)
	if e != nil {
		return nil, errors.Join(e, s.releaseSturmCoefficients(a))
	}
	defer func() { err = errors.Join(err, s.releaseSturmCoefficients(a), s.releaseSturmCoefficients(b)) }()
	for len(b) > 0 {
		q, r, x := s.divideAnyPolynomials(a, b, usePrem)
		x = errors.Join(x, s.releaseSturmCoefficients(q))
		if x != nil {
			_ = s.releaseSturmCoefficients(r)
			return nil, x
		}
		if x = s.releaseSturmCoefficients(a); x != nil {
			_ = s.releaseSturmCoefficients(r)
			a = nil
			return nil, x
		}
		a, b = b, r
	}
	return s.makeAnyPolynomialMonic(a, usePrem)
}

// flipAnySignIfLeadingNegative is flip_sign_if_lc_neg. It takes ownership
// of p and returns the owned result.
func (s *realValueStore) flipAnySignIfLeadingNegative(p []*realValue, usePrem bool) ([]*realValue, error) {
	if len(p) == 0 {
		return p, nil
	}
	sign, err := s.determineRankedRetainedValueSign(p[len(p)-1], usePrem)
	if err != nil || sign >= 0 {
		if err != nil {
			err = errors.Join(err, s.releaseSturmCoefficients(p))
			return nil, err
		}
		return p, nil
	}
	negated, err := s.negateAnyPolynomial(p, usePrem)
	err = errors.Join(err, s.releaseSturmCoefficients(p))
	if err != nil {
		return nil, errors.Join(err, s.releaseSturmCoefficients(negated))
	}
	return negated, nil
}

// normalizeAnyIntCoeffsOwned is normalize_int_coeffs on an owned buffer.
func (s *realValueStore) normalizeAnyIntCoeffsOwned(p []*realValue, usePrem bool) ([]*realValue, error) {
	if len(p) == 0 {
		// gcd_int_coeffs finds no integer content in the zero polynomial.
		return p, nil
	}
	normalized, _, err := s.normalizeAnyIntegerContent(p, usePrem)
	err = errors.Join(err, s.releaseSturmCoefficients(p))
	if err != nil {
		return nil, errors.Join(err, s.releaseSturmCoefficients(normalized))
	}
	return normalized, nil
}

// premGcdAnyPolynomials is prem_gcd (realclosure.cpp:3542-3581): the gcd
// by plain pseudo-remainders, normalizing integer content after each step,
// with a nonnegative leading coefficient.
func (s *realValueStore) premGcdAnyPolynomials(left, right []*realValue, usePrem bool) (result []*realValue, err error) {
	if len(left) == 0 || len(right) == 0 {
		other := right
		if len(right) == 0 {
			other = left
		}
		r, e := s.retainRationalPolynomial(other)
		if e != nil {
			return nil, e
		}
		return s.flipAnySignIfLeadingNegative(r, usePrem)
	}
	a, e := s.retainRationalPolynomial(left)
	if e != nil {
		return nil, e
	}
	b, e := s.retainRationalPolynomial(right)
	if e != nil {
		return nil, errors.Join(e, s.releaseSturmCoefficients(a))
	}
	defer func() { err = errors.Join(err, s.releaseSturmCoefficients(a), s.releaseSturmCoefficients(b)) }()
	for {
		if len(b) == 0 {
			normalized, e := s.normalizeAnyIntCoeffsOwned(a, usePrem)
			a = nil
			if e != nil {
				return nil, e
			}
			return s.flipAnySignIfLeadingNegative(normalized, usePrem)
		}
		r, e := s.pseudoRemainderAny(a, b, usePrem)
		if e != nil {
			return nil, e
		}
		r, e = s.normalizeAnyIntCoeffsOwned(r, usePrem)
		if e != nil {
			return nil, e
		}
		if e = s.releaseSturmCoefficients(a); e != nil {
			a = nil
			_ = s.releaseSturmCoefficients(r)
			return nil, e
		}
		a, b = b, r
	}
}
