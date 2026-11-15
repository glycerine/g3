package realclosure

// Source: src/math/realclosure/realclosure.cpp:4808-4865.

import "errors"

func (s *realValueStore) negateRankedRetainedValue(value *realValue, usePrem bool) (result *realValue, err error) {
	if value == nil || value.isRational {
		return s.negateRational(value)
	}
	numerator := make([]*realValue, len(value.functionNumerator.coefficients))
	defer func() { err = errors.Join(err, s.releaseSturmCoefficients(numerator)) }()
	for i, coefficient := range value.functionNumerator.coefficients {
		numerator[i], err = s.negateRankedRetainedValue(coefficient, usePrem)
		if err != nil {
			return nil, err
		}
	}
	approximation, err := NewBinaryInterval(value.root.owner.intervals)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, approximation.Close()) }()
	if err = approximation.Neg(value.functionInterval); err != nil {
		return nil, err
	}
	// neg_rf negates only the numerator and keeps the denominator ad; it
	// does not call determine_sign, since -interval(a) excludes zero.
	return s.newRankedRationalFunctionValue(value.root, numerator, value.functionDenominator.coefficients, approximation)
}
