package realclosure

// Source: src/math/realclosure/realclosure.cpp:2879-2913, 2991-3030.

import "errors"

func (s *realValueStore) signedRemainderRanked(left, right []*realValue, usePrem bool) (result []*realValue, err error) {
	quotient, remainder, err := s.divideRankedPolynomials(left, right, usePrem)
	if err != nil {
		return nil, err
	}
	if err = s.releaseSturmCoefficients(quotient); err != nil {
		return nil, errors.Join(err, s.releaseSturmCoefficients(remainder))
	}
	result = make([]*realValue, len(remainder))
	defer func() {
		err = errors.Join(err, s.releaseSturmCoefficients(remainder))
		if err != nil {
			err = errors.Join(err, s.releaseSturmCoefficients(result))
			result = nil
		}
	}()
	for i, coefficient := range remainder {
		result[i], err = s.negateRankedRetainedValue(coefficient, usePrem)
		if err != nil {
			return nil, err
		}
	}
	return trimAnyPolynomial(result), nil
}
