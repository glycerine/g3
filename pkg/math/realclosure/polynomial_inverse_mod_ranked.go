package realclosure

// Source: src/math/realclosure/realclosure.cpp:5333-5403
// (inv_algebraic(q_sz, q, p_sz, p, g, h)).

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

// invertRankedPolynomialModulo finds h with q(alpha)*h(alpha) = 1 where
// p(alpha) = 0, by rewriting Q(alpha)*h(alpha) = R(alpha) from Q = q, R = 1
// until Q is a constant. The modulus p never changes. When some Q divides p,
// it fails with ok = false and returns g = monic(Q), a factor of p that the
// caller uses to refine the defining polynomial of alpha.
func (s *realValueStore) invertRankedPolynomialModulo(q, p []*realValue, usePrem bool) (h, g []*realValue, ok bool, err error) {
	if err = s.checkAnyPolynomial(q, false); err != nil {
		return nil, nil, false, err
	}
	if err = s.checkAnyPolynomial(p, false); err != nil || len(q) == 0 || len(q) >= len(p) {
		if err == nil {
			err = arena.ErrInvalidSlot
		}
		return nil, nil, false, err
	}
	// Q <- q
	Q, err := s.retainRationalPolynomial(q)
	if err != nil {
		return nil, nil, false, err
	}
	// R <- 1
	one, err := s.newRational(1, 1)
	if err != nil {
		return nil, nil, false, errors.Join(err, s.releaseSturmCoefficients(Q))
	}
	R := []*realValue{one}
	defer func() { err = errors.Join(err, s.releaseSturmCoefficients(Q), s.releaseSturmCoefficients(R)) }()
	for {
		if err = s.checkpoint(); err != nil {
			return nil, nil, false, err
		}
		// In every iteration Q(alpha) * h(alpha) = R(alpha).
		if len(Q) == 1 {
			// h(alpha) = R(alpha) / Q[0]
			h, err = s.divideRankedPolynomialByScalar(R, Q[0], usePrem)
			if err != nil {
				return nil, nil, false, err
			}
			return h, nil, true, nil
		}
		quotient, remainder, e := s.divideRankedPolynomials(p, Q, usePrem)
		if e != nil {
			return nil, nil, false, e
		}
		if len(remainder) == 0 {
			// GCD(q, p) != 1
			g, e = s.makeRankedPolynomialMonic(Q, usePrem)
			e = errors.Join(e, s.releaseSturmCoefficients(quotient))
			if e != nil {
				return nil, nil, false, errors.Join(e, s.releaseSturmCoefficients(g))
			}
			return nil, g, false, nil
		}
		// p == Quo * Q + Rem and p(alpha) = 0, so Quo(alpha) * Q(alpha) =
		// -Rem(alpha): Q <- -Rem, R <- rem(R * Quo, p).
		negated, e := s.negateAnyPolynomial(remainder, usePrem)
		e = errors.Join(e, s.releaseSturmCoefficients(remainder))
		if e != nil {
			return nil, nil, false, errors.Join(e, s.releaseSturmCoefficients(quotient), s.releaseSturmCoefficients(negated))
		}
		if e = s.releaseSturmCoefficients(Q); e != nil {
			Q = negated
			return nil, nil, false, errors.Join(e, s.releaseSturmCoefficients(quotient))
		}
		Q = negated
		aux, e := s.multiplyRankedPolynomials(R, quotient, usePrem)
		e = errors.Join(e, s.releaseSturmCoefficients(quotient))
		if e != nil {
			return nil, nil, false, errors.Join(e, s.releaseSturmCoefficients(aux))
		}
		discarded, reduced, e := s.divideRankedPolynomials(aux, p, usePrem)
		e = errors.Join(e, s.releaseSturmCoefficients(aux), s.releaseSturmCoefficients(discarded))
		if e != nil {
			return nil, nil, false, errors.Join(e, s.releaseSturmCoefficients(reduced))
		}
		if e = s.releaseSturmCoefficients(R); e != nil {
			R = reduced
			return nil, nil, false, e
		}
		R = reduced
	}
}
