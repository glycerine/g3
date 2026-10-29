package realclosure

// Source: src/math/realclosure/realclosure.cpp:2879-2913, 2991-3030.

import "errors"

// signedRemainderAny computes -(left mod right) over the retained value field.
func (s *realValueStore) signedRemainderAny(left, right []*realValue, usePrem bool) (result []*realValue, err error) {
	quotient, remainder, err := s.divideAnyPolynomials(left, right, usePrem)
	if err != nil {
		return nil, err
	}
	if err = s.releaseSturmCoefficients(quotient); err != nil {
		return nil, errors.Join(err, s.releaseSturmCoefficients(remainder))
	}
	result, err = s.negateAnyPolynomial(remainder)
	err = errors.Join(err, s.releaseSturmCoefficients(remainder))
	if err != nil {
		_ = s.releaseSturmCoefficients(result)
		return nil, err
	}
	return result, nil
}
