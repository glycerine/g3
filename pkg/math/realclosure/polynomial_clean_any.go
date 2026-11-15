package realclosure

// Source: src/math/realclosure/realclosure.cpp:3194-3335
// (clean_denominators and clean_denominators_core on polynomials).

// cleanAnyPolynomialDenominators is clean_denominators(sz, p, norm_p, d), as
// nz_isolate_roots calls it: a polynomial with clean denominators is returned
// unchanged; otherwise both rational and non-rational denominators are
// cleaned.
func (s *realValueStore) cleanAnyPolynomialDenominators(p []*realValue, usePrem bool) (normalized []*realValue, denominator *realValue, err error) {
	return s.cleanTowerPolynomialDenominatorsIfNeeded(p, usePrem)
}
