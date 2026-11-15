package realclosure

// Source: src/math/realclosure/realclosure.cpp:4108-4147 (polynomial_interval,
// update_rf_interval). polynomial_interval takes interval(p[i]) for
// coefficients of every kind, not only rationals.

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) updateAlgebraicPolynomialValueInterval(value *realValue) error {
	if value == nil || value.isRational || value.root == nil || value.functionInterval == nil || value.root.interval == nil {
		return arena.ErrInvalidSlot
	}
	return s.evaluateRankedPolynomialInterval(value.functionNumerator.coefficients, value.root.interval, value.functionInterval)
}
