package realclosure

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// invertAlgebraicPolynomial is inv_algebraic's Q/R recurrence: it returns
// h with q*h = 1 modulo p, or false when gcd(q, p) != 1. The current
// retained algebraic extensions have rational defining coefficients. The
// result is caller-owned; q and p stay owned by the caller.
func (s *realValueStore) invertAlgebraicPolynomial(q, p []numeral.Rational) (h []numeral.Rational, ok bool, err error) {
	if len(q) == 0 || len(q) >= len(p) {
		return nil, false, nil
	}
	// Q <- q, R <- 1
	Q, err := s.qPolyCopy(q)
	if err != nil {
		return nil, false, err
	}
	R := newQPoly(1)
	defer func() { err = errors.Join(err, s.qPolyDel(R), s.qPolyDel(Q)) }()
	if err = s.numbers.SetInt64(&R[0], 1); err != nil {
		return nil, false, err
	}
	for {
		if len(Q) == 1 {
			// h <- R / Q[0]
			h, err = s.qPolyDiv(R, &Q[0])
			return h, err == nil, err
		}
		quotient, remainder, divided, e := s.qPolyDivRem(p, Q)
		if e != nil {
			return nil, false, e
		}
		if !divided || len(remainder) == 0 {
			return nil, false, errors.Join(s.qPolyDel(quotient), s.qPolyDel(remainder))
		}
		// Q <- -Rem
		negated, e := s.qPolyNeg(remainder)
		if e = errors.Join(e, s.qPolyDel(remainder)); e != nil {
			return nil, false, errors.Join(e, s.qPolyDel(quotient), s.qPolyDel(negated))
		}
		if e = s.qPolyDel(Q); e != nil {
			return nil, false, errors.Join(e, s.qPolyDel(quotient), s.qPolyDel(negated))
		}
		Q = negated
		// aux <- R * Quo; R <- rem(aux, p)
		aux, e := s.qPolyMul(R, quotient)
		if e = errors.Join(e, s.qPolyDel(quotient)); e != nil {
			return nil, false, errors.Join(e, s.qPolyDel(aux))
		}
		reduced, reducedOK, e := s.qPolyRem(aux, p)
		if e = errors.Join(e, s.qPolyDel(aux)); e != nil {
			return nil, false, errors.Join(e, s.qPolyDel(reduced))
		}
		if e = s.qPolyDel(R); e != nil {
			return nil, false, errors.Join(e, s.qPolyDel(reduced))
		}
		R = reduced
		if !reducedOK {
			return nil, false, nil
		}
	}
}

func (s *realValueStore) inverseNormalizedValue(a *realValue) (*realValue, error) {
	if !s.realOperand(a) || a == nil {
		return nil, arena.ErrInvalidSlot
	}
	if a.isRational {
		one, err := s.newRational(1, 1)
		if err != nil {
			return nil, err
		}
		out, err := s.divideRationals(one, a)
		releaseErr := s.release(one)
		if err != nil {
			return nil, err
		}
		if releaseErr != nil {
			_ = s.release(out)
			return nil, releaseErr
		}
		return out, nil
	}
	values, err := s.inverseAlgebraicCoefficients(a)
	if err != nil {
		return nil, err
	}
	approximation, err := s.newApproximation(a.root)
	if err != nil {
		_ = s.releaseTemporaryCoefficients(values)
		return nil, err
	}
	if err = approximation.Inv(a.functionInterval, 32); err != nil {
		_ = approximation.Close()
		_ = s.releaseTemporaryCoefficients(values)
		return nil, err
	}
	out, err := s.publishFunctionValue(a.root, values, approximation)
	closeErr := approximation.Close()
	releaseErr := s.releaseTemporaryCoefficients(values)
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		_ = s.release(out)
		return nil, closeErr
	}
	if releaseErr != nil {
		_ = s.release(out)
		return nil, releaseErr
	}
	return out, nil
}

// inverseAlgebraicCoefficients is inv_algebraic(a, r)'s new_num: norm_q =
// rem(q, p), then inv_algebraic(norm_q, p), published as rational values.
func (s *realValueStore) inverseAlgebraicCoefficients(a *realValue) (values []*realValue, err error) {
	q, err := s.rationalPolynomialValues(a.functionNumerator.coefficients)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.qPolyDel(q)) }()
	p, err := s.rationalPolynomialValues(a.root.polynomial.coefficients)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.qPolyDel(p)) }()
	normQ, ok, err := s.qPolyRem(q, p)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.qPolyDel(normQ)) }()
	if !ok || len(normQ) == 0 {
		return nil, arena.ErrInvalidSlot
	}
	h, ok, err := s.invertAlgebraicPolynomial(normQ, p)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.qPolyDel(h)) }()
	if !ok {
		return nil, arena.ErrInvalidSlot
	}
	return s.ownRationalPolynomial(h)
}
