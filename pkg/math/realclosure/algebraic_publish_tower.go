package realclosure

// Source: src/math/realclosure/realclosure.cpp:1220-1275, 3130-3165.

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) publishRankedAlgebraicPolynomialResult(root *algebraicRoot, coefficients []*realValue, approximation *BinaryInterval) (*realValue, error) {
	if root == nil || approximation == nil || len(coefficients) == 0 {
		return nil, arena.ErrInvalidSlot
	}
	if len(coefficients) == 1 {
		if err := s.retain(coefficients[0]); err != nil {
			return nil, err
		}
		return coefficients[0], nil
	}
	return s.newRankedAlgebraicPolynomialValue(root, coefficients, approximation)
}
