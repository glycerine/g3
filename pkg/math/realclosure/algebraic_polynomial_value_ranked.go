package realclosure

// Source: src/math/realclosure/realclosure.cpp:1180-1275, 4923-4943.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *realValueStore) newRankedAlgebraicPolynomialValue(root *algebraicRoot, numerator []*realValue, approximation *BinaryInterval) (value *realValue, err error) {
	if s == nil || root == nil || !root.isRegistered() || root.owner.values != s || approximation == nil || approximation.numbers != root.owner.intervals || len(numerator) == 0 || (root.kind == realExtensionAlgebraic && len(numerator) >= len(root.polynomial.coefficients)) || numerator[len(numerator)-1] == nil {
		return nil, arena.ErrInvalidSlot
	}
	for _, coefficient := range numerator {
		if coefficient == nil {
			continue
		}
		if !s.valueHasLowerExtensionRank(coefficient, root) {
			return nil, arena.ErrInvalidSlot
		}
	}
	if err = root.retain(); err != nil {
		return nil, err
	}
	value = &realValue{store: s, root: root, refs: 1}
	defer func() {
		if err != nil {
			if value.functionInterval != nil {
				err = errors.Join(err, value.functionInterval.Close())
			}
			err = errors.Join(err, s.resetPolynomial(&value.functionNumerator), root.release())
			value = nil
		}
	}()
	if err = s.setPolynomial(&value.functionNumerator, numerator); err != nil {
		return nil, err
	}
	value.functionInterval, err = NewBinaryInterval(root.owner.intervals)
	if err != nil {
		return nil, err
	}
	err = value.functionInterval.SetFrom(approximation)
	return value, err
}

func (s *realValueStore) valueHasLowerExtensionRank(value *realValue, root *algebraicRoot) bool {
	if value == nil {
		return true
	}
	if _, err := s.retainedValueRank(value); err != nil {
		return false
	}
	if value.isRational {
		return true
	}
	left, right := sourceExtensionRank(value.root.kind), sourceExtensionRank(root.kind)
	return left < right || left == right && value.root.index < root.index
}
