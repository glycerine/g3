package realclosure

// Source: src/math/realclosure/realclosure.cpp:3149-3191
// (clean_denominators_core(value*)), 1269-1275 (mk_rational_function_value)
// and 3787-3807 (mk_polynomial_value).

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

// cleanTowerValueDenominator is clean_denominators_core(a, p, q): it returns
// owned p and q with a == p/q whose coefficients have clean denominators.
func (s *realValueStore) cleanTowerValueDenominator(value *realValue, usePrem bool) (numerator, denominator *realValue, err error) {
	if value == nil || value.isRational {
		return s.cleanRationalValueDenominator(value)
	}
	root := value.root
	if root == nil || !root.isRegistered() {
		return nil, nil, arena.ErrInvalidSlot
	}
	var pNum, pDen []*realValue
	var dNum, dDen *realValue
	defer func() {
		err = errors.Join(err,
			s.releaseSturmCoefficients(pNum), s.releaseSturmCoefficients(pDen),
			s.releaseSturmScratch(dNum), s.releaseSturmScratch(dDen))
		if err != nil {
			err = errors.Join(err, s.releaseSturmScratch(numerator), s.releaseSturmScratch(denominator))
			numerator, denominator = nil, nil
		}
	}()
	pNum, dNum, err = s.cleanTowerPolynomialDenominators(value.functionNumerator.coefficients, usePrem)
	if err != nil {
		return nil, nil, err
	}
	if root.kind == realExtensionAlgebraic || s.rankedDenominatorIsOne(value.functionDenominator.coefficients) {
		one, e := s.newRational(1, 1)
		if e != nil {
			return nil, nil, e
		}
		pDen = []*realValue{one}
		if dDen, err = s.newRational(1, 1); err != nil {
			return nil, nil, err
		}
	} else {
		pDen, dDen, err = s.cleanTowerPolynomialDenominators(value.functionDenominator.coefficients, usePrem)
		if err != nil {
			return nil, nil, err
		}
	}
	x, err := s.newExtensionGeneratorValue(root)
	if err != nil {
		return nil, nil, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmScratch(x)) }()
	if numerator, err = s.rankedPolynomialValueAt(pNum, x, usePrem); err != nil {
		return nil, nil, err
	}
	if denominator, err = s.rankedPolynomialValueAt(pDen, x, usePrem); err != nil {
		return nil, nil, err
	}
	equal, err := s.structurallyEqualValues(dDen, dNum)
	if err != nil {
		return nil, nil, err
	}
	if !equal {
		if numerator, err = s.replaceRankedProduct(numerator, dDen, usePrem); err != nil {
			return nil, nil, err
		}
		if denominator, err = s.replaceRankedProduct(denominator, dNum, usePrem); err != nil {
			return nil, nil, err
		}
	}
	sign, err := s.determineRankedRetainedValueSign(denominator, usePrem)
	if err != nil {
		return nil, nil, err
	}
	if sign < 0 {
		// make sure the denominator is positive
		if numerator, err = s.replaceRankedNegation(numerator, usePrem); err != nil {
			return nil, nil, err
		}
		if denominator, err = s.replaceRankedNegation(denominator, usePrem); err != nil {
			return nil, nil, err
		}
	}
	return numerator, denominator, nil
}

// replaceRankedProduct is mul(a, b, a): it consumes the reference to a.
func (s *realValueStore) replaceRankedProduct(a, b *realValue, usePrem bool) (*realValue, error) {
	product, err := s.multiplyRankedRetainedValues(a, b, usePrem)
	if err != nil {
		return a, err
	}
	return product, s.releaseSturmScratch(a)
}

// replaceRankedNegation is neg(a, a): it consumes the reference to a.
func (s *realValueStore) replaceRankedNegation(a *realValue, usePrem bool) (*realValue, error) {
	negated, err := s.negateRankedRetainedValue(a, usePrem)
	if err != nil {
		return a, err
	}
	return negated, s.releaseSturmScratch(a)
}

// newExtensionGeneratorValue is mk_rational_function_value(ext): the value
// x/1 over the extension, with the extension's interval.
func (s *realValueStore) newExtensionGeneratorValue(root *algebraicRoot) (value *realValue, err error) {
	if root == nil || root.interval == nil {
		return nil, arena.ErrInvalidSlot
	}
	one, err := s.newRational(1, 1)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmScratch(one)) }()
	return s.newRankedAlgebraicPolynomialValue(root, []*realValue{nil, one}, root.interval)
}

// rankedPolynomialValueAt is mk_polynomial_value(n, p, b, r): the Horner
// sequence ((a_{n-1}*b + a_{n-2})*b + ...)*b + a_0 with the value arithmetic.
func (s *realValueStore) rankedPolynomialValueAt(p []*realValue, b *realValue, usePrem bool) (result *realValue, err error) {
	if len(p) == 0 {
		return nil, arena.ErrInvalidSlot
	}
	if len(p) == 1 || b == nil {
		if err = s.retain(p[0]); err != nil {
			return nil, err
		}
		return p[0], nil
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, s.releaseSturmScratch(result))
			result = nil
		}
	}()
	if result, err = s.multiplyRankedRetainedValues(p[len(p)-1], b, usePrem); err != nil {
		return nil, err
	}
	for i := len(p) - 1; i > 0; {
		i--
		if p[i] != nil {
			sum, e := s.addRankedRetainedValues(result, p[i], usePrem)
			if e != nil {
				return result, e
			}
			if e = s.releaseSturmScratch(result); e != nil {
				return sum, e
			}
			result = sum
		}
		if i > 0 {
			if result, err = s.replaceRankedProduct(result, b, usePrem); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}
