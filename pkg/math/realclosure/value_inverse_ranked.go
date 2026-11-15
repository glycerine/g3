package realclosure

// Source: src/math/realclosure/realclosure.cpp:5330-5585.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) inverseRankedRetainedValue(value *realValue, usePrem bool) (result *realValue, err error) {
	if value == nil || value.store != s || value.deleted || value.refs == 0 {
		return nil, arena.ErrInvalidSlot
	}
	if value.isRational {
		return s.inverseRetainedValue(value)
	}
	root := value.root
	if root == nil || !root.isRegistered() {
		return nil, arena.ErrInvalidSlot
	}
	if root.kind != realExtensionAlgebraic {
		if len(value.functionDenominator.coefficients) == 0 {
			return nil, arena.ErrInvalidSlot
		}
		approximation, e := NewBinaryInterval(root.owner.intervals)
		if e != nil {
			return nil, e
		}
		defer func() { err = errors.Join(err, approximation.Close()) }()
		if err = approximation.Inv(value.functionInterval, s.rationalInitialPrecision()); err != nil {
			return nil, err
		}
		numerator, denominator, e := s.normalizeRankedFraction(value.functionDenominator.coefficients, value.functionNumerator.coefficients, usePrem)
		if e != nil {
			return nil, e
		}
		defer func() {
			err = errors.Join(err, s.releaseSturmCoefficients(numerator), s.releaseSturmCoefficients(denominator))
		}()
		result, err = s.newRankedRationalFunctionValue(root, numerator, denominator, approximation)
		if err != nil {
			return nil, err
		}
		return result, nil
	}
	return s.inverseRankedAlgebraicValue(value, usePrem)
}

// inverseRankedAlgebraicValue is inv_algebraic(a, r). When q = a->num() and
// the defining polynomial p of alpha are not co-prime, the source refines
// alpha with p/g, where g is the factor inv_algebraic reports, and retries.
func (s *realValueStore) inverseRankedAlgebraicValue(value *realValue, usePrem bool) (result *realValue, err error) {
	root := value.root
	q := value.functionNumerator.coefficients
	// since p(alpha) = 0, q(alpha) = rem(q, p)(alpha)
	normalized, err := s.reduceRankedAlgebraicPolynomial(root, q, usePrem)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmCoefficients(normalized)) }()
	inverse, g, ok, err := s.invertRankedPolynomialModulo(normalized, root.polynomial.coefficients, usePrem)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmCoefficients(inverse), s.releaseSturmCoefficients(g)) }()
	if !ok {
		return s.inverseRankedAlgebraicRefined(value, g, usePrem)
	}
	if len(inverse) == 0 {
		return nil, arena.ErrInvalidSlot
	}
	if len(inverse) == 1 {
		result = inverse[0]
		inverse[0] = nil
		return result, nil
	}
	approximation, err := NewBinaryInterval(root.owner.intervals)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, approximation.Close()) }()
	if err = approximation.Inv(value.functionInterval, s.rationalInitialPrecision()); err != nil {
		return nil, err
	}
	result, err = s.newRankedAlgebraicPolynomialValue(root, inverse, approximation)
	if err != nil {
		return nil, err
	}
	return s.canonicalizeRankedRetainedResult(result, usePrem)
}

// inverseRankedAlgebraicRefined is the failure branch of inv_algebraic(a, r):
// q and p are not co-prime because the defining polynomial of alpha need not
// be minimal. Refine alpha with new_p = p/g and try again.
func (s *realValueStore) inverseRankedAlgebraicRefined(value *realValue, g []*realValue, usePrem bool) (result *realValue, err error) {
	root := value.root
	owner := root.owner
	newP, remainder, err := s.divideRankedPolynomials(root.polynomial.coefficients, g, usePrem)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmCoefficients(newP)) }()
	if err = s.releaseSturmCoefficients(remainder); err != nil {
		return nil, err
	}
	if owner.api == nil || owner.api.cleanDenominators {
		cleaned, d, e := s.cleanTowerPolynomialDenominatorsIfNeeded(newP, usePrem)
		e = errors.Join(e, s.releaseSturmScratch(d))
		if e != nil {
			return nil, errors.Join(e, s.releaseSturmCoefficients(cleaned))
		}
		if e = s.releaseSturmCoefficients(newP); e != nil {
			newP = cleaned
			return nil, e
		}
		newP = cleaned
	}
	if len(newP) < 2 || newP[0] == nil || newP[len(newP)-1] == nil {
		return nil, arena.ErrInvalidSlot
	}
	if len(newP) == 2 {
		// Easy case: alpha is actually equal to -new_p[0]/new_p[1], and a is
		// q(alpha_val), which does not depend on alpha anymore.
		negated, e := s.negateRankedRetainedValue(newP[0], usePrem)
		if e != nil {
			return nil, e
		}
		alphaValue, e := s.divideRankedRetainedValues(negated, newP[1], usePrem)
		e = errors.Join(e, s.releaseSturmScratch(negated))
		if e != nil {
			return nil, errors.Join(e, s.releaseSturmScratch(alphaValue))
		}
		defer func() { err = errors.Join(err, s.releaseSturmScratch(alphaValue)) }()
		newA, e := s.rankedPolynomialValueAt(value.functionNumerator.coefficients, alphaValue, usePrem)
		if e != nil {
			return nil, e
		}
		defer func() { err = errors.Join(err, s.releaseSturmScratch(newA)) }()
		if newA == nil {
			return nil, arena.ErrInvalidSlot
		}
		return s.inverseRankedRetainedValue(newA, usePrem)
	}
	if root.signs == nil {
		// Another easy case: replace alpha->p() with new_p. The isolating
		// interval of p is also isolating for new_p, whose roots are a
		// subset of the roots of p.
		if err = s.resetPolynomial(&root.polynomial); err != nil {
			return nil, err
		}
		if err = s.setPolynomial(&root.polynomial, newP); err != nil {
			return nil, err
		}
		// The new call succeeds because q and new_p are co-prime.
		return s.inverseRankedAlgebraicValue(value, usePrem)
	}
	// Isolate the roots of new_p (nonlinear, square free, 0 is not a root)
	// and copy the fields of the one equal to alpha.
	roots, err := owner.isolateNonlinearSquareFreeAnyRoots(newP, usePrem)
	defer func() { err = errors.Join(err, s.releaseSturmCoefficients(roots)) }()
	if err != nil {
		return nil, err
	}
	if len(roots) == 0 {
		return nil, arena.ErrInvalidSlot
	}
	var newAlpha *algebraicRoot
	if len(roots) == 1 {
		newAlpha = roots[0].root
	} else {
		alphaValue, e := s.newExtensionGeneratorValue(root)
		if e != nil {
			return nil, e
		}
		defer func() { err = errors.Join(err, s.releaseSturmScratch(alphaValue)) }()
		i := 0
		for ; i < len(roots); i++ {
			comparison, e := s.compareRankedRetainedValues(alphaValue, roots[i], usePrem)
			if e != nil {
				return nil, e
			}
			if comparison == 0 {
				break
			}
		}
		if i == len(roots) {
			return nil, arena.ErrInvalidSlot
		}
		newAlpha = roots[i].root
	}
	if newAlpha == nil || newAlpha.kind != realExtensionAlgebraic || len(newAlpha.polynomial.coefficients) != len(newP) {
		return nil, arena.ErrInvalidSlot
	}
	// copy new_alpha->m_p
	if err = s.resetPolynomial(&root.polynomial); err != nil {
		return nil, err
	}
	if err = s.setPolynomial(&root.polynomial, newAlpha.polynomial.coefficients); err != nil {
		return nil, err
	}
	// copy new_alpha->m_sign_det
	if newAlpha.signs != nil {
		if err = newAlpha.signs.retain(); err != nil {
			return nil, err
		}
	}
	if root.signs != nil {
		if err = root.signs.release(); err != nil {
			return nil, err
		}
	}
	root.signs = newAlpha.signs
	// copy remaining fields
	if err = root.isolating.SetFrom(newAlpha.isolating); err != nil {
		return nil, err
	}
	root.signIndex = newAlpha.signIndex
	root.dependsOnInfinitesimals = newAlpha.dependsOnInfinitesimals
	// The new call succeeds because q and new_p are co-prime.
	return s.inverseRankedAlgebraicValue(value, usePrem)
}
