package realclosure

// Source: src/math/realclosure/realclosure.cpp:4460-4766.

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) determineRankedRetainedValueSign(value *realValue, usePrem bool) (int32, error) {
	if value == nil || value.isRational {
		return s.signValue(value)
	}
	if value.root == nil || value.root.deleted || value.functionInterval == nil {
		return 0, arena.ErrInvalidSlot
	}
	contains, err := value.functionInterval.ContainsZero()
	if err != nil {
		return 0, err
	}
	if !contains {
		return s.signValue(value)
	}
	if value.root.kind == realExtensionTranscendental {
		for precision := uint32(1); ; precision++ {
			if _, err := s.refineTranscendentalPolynomialValue(value, precision); err != nil {
				return 0, err
			}
			contains, err = value.functionInterval.ContainsZero()
			if err != nil {
				return 0, err
			}
			if !contains {
				return s.signValue(value)
			}
			if precision == ^uint32(0) {
				return 0, arena.ErrBounds
			}
		}
	}
	if value.root.signs != nil {
		return s.determineAnyRetainedValueSign(value, usePrem)
	}
	sign, err := s.tarskiBinaryIntervalAny(value.root.polynomial.coefficients, value.functionNumerator.coefficients, value.root.isolating, usePrem)
	if err != nil || sign == 0 {
		return int32(sign), err
	}
	result := int32(1)
	if sign < 0 {
		result = -1
	}
	if err = restrictBinaryIntervalToSign(value.functionInterval, result); err != nil {
		return 0, err
	}
	return result, nil
}
