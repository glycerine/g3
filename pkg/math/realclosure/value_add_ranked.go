package realclosure

// Source: src/math/realclosure/realclosure.cpp:4867-5055.

import (
	"errors"
	"fmt"
)

func (s *realValueStore) addRankedRetainedValues(left, right *realValue, usePrem bool) (result *realValue, err error) {
	comparison, err := s.compareRetainedValueRanks(left, right)
	if err != nil {
		return nil, err
	}
	if left == nil {
		if err = s.retain(right); err != nil {
			return nil, err
		}
		return right, nil
	}
	if right == nil {
		if err = s.retain(left); err != nil {
			return nil, err
		}
		return left, nil
	}
	if left.isRational && right.isRational {
		return s.addRationals(left, right)
	}
	higher, lower := left, right
	if comparison < 0 {
		higher, lower = right, left
	}
	root := higher.root
	leftCoefficients := higher.functionNumerator.coefficients
	rightCoefficients := []*realValue{lower}
	if comparison == 0 {
		leftCoefficients = left.functionNumerator.coefficients
		rightCoefficients = right.functionNumerator.coefficients
	}
	size := len(leftCoefficients)
	if len(rightCoefficients) > size {
		size = len(rightCoefficients)
	}
	numerator := make([]*realValue, size)
	defer func() { err = errors.Join(err, s.releaseSturmCoefficients(numerator)) }()
	for i := 0; i < size; i++ {
		var a, b *realValue
		if i < len(leftCoefficients) {
			a = leftCoefficients[i]
		}
		if i < len(rightCoefficients) {
			b = rightCoefficients[i]
		}
		numerator[i], err = s.addRankedRetainedValues(a, b, usePrem)
		if err != nil {
			return nil, err
		}
	}
	numerator = trimAnyPolynomial(numerator)
	if len(numerator) == 0 {
		return nil, nil
	}
	if len(numerator) == 1 {
		result = numerator[0]
		numerator[0] = nil
		return result, nil
	}
	approximation, err := NewBinaryInterval(root.owner.intervals)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, approximation.Close()) }()
	leftInterval, err := s.valueInterval(left, root.owner.intervals)
	if err != nil {
		return nil, err
	}
	rightInterval, err := s.valueInterval(right, root.owner.intervals)
	if err != nil {
		return nil, err
	}
	if err = approximation.Add(leftInterval, rightInterval); err != nil {
		return nil, err
	}
	result, err = s.newRankedAlgebraicPolynomialValue(root, numerator, approximation)
	if err != nil {
		return nil, err
	}
	result, err = s.canonicalizeRankedRetainedResult(result, usePrem)
	if err != nil {
		return result, fmt.Errorf("realclosure: canonicalize extension sum: %w", err)
	}
	return result, nil
}
