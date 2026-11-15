package realclosure

// Source: src/math/realclosure/realclosure.cpp:4923-5159.

// The "any" polynomial helpers reach coefficients of every extension kind
// (algebraic, transcendental and infinitesimal), so they use the generic
// value arithmetic, as the source's add/sub/mul do.

func (s *realValueStore) addAnyCanonicalValues(left, right *realValue, usePrem bool) (*realValue, error) {
	return s.addRankedRetainedValues(left, right, usePrem)
}

func (s *realValueStore) subtractAnyCanonicalValues(left, right *realValue, usePrem bool) (*realValue, error) {
	return s.subtractRankedRetainedValues(left, right, usePrem)
}

func (s *realValueStore) multiplyAnyCanonicalValues(left, right *realValue, usePrem bool) (*realValue, error) {
	return s.multiplyRankedRetainedValues(left, right, usePrem)
}
