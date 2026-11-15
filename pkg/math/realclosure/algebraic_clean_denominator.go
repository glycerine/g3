package realclosure

// Source: src/math/realclosure/realclosure.cpp:3149-3191.

// cleanAnyValueDenominator is clean_denominators_core(value*): it cleans the
// numerator and the denominator of a rational-function value of any
// extension kind.
func (s *realValueStore) cleanAnyValueDenominator(value *realValue, usePrem bool) (numerator, denominator *realValue, err error) {
	return s.cleanTowerValueDenominator(value, usePrem)
}
