package realclosure

// Source: src/math/realclosure/realclosure.cpp:5163-5318.

import "errors"

func (s *realValueStore) multiplyRankedRetainedValues(left, right *realValue, usePrem bool) (result *realValue, err error) {
	comparison, err := s.compareRetainedValueRanks(left, right)
	if err != nil {
		return nil, err
	}
	if left == nil || right == nil {
		return nil, nil
	}
	if left.isRational && right.isRational {
		return s.multiplyRationals(left, right)
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
	product := make([]*realValue, len(leftCoefficients)+len(rightCoefficients)-1)
	defer func() { err = errors.Join(err, s.releaseSturmCoefficients(product)) }()
	for i, a := range leftCoefficients {
		for j, b := range rightCoefficients {
			term, e := s.multiplyRankedRetainedValues(a, b, usePrem)
			if e != nil {
				return nil, e
			}
			updated, e := s.addRankedRetainedValues(product[i+j], term, usePrem)
			e = errors.Join(e, s.releaseSturmScratch(term))
			if e != nil {
				return nil, e
			}
			if e = s.releaseSturmScratch(product[i+j]); e != nil {
				_ = s.releaseSturmScratch(updated)
				return nil, e
			}
			product[i+j] = updated
		}
	}
	product = trimAnyPolynomial(product)
	reduced, err := s.reduceRankedAlgebraicPolynomial(root, product, usePrem)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, s.releaseSturmCoefficients(reduced)) }()
	if len(reduced) == 0 {
		return nil, nil
	}
	if len(reduced) == 1 {
		result = reduced[0]
		reduced[0] = nil
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
	if err = approximation.Mul(leftInterval, rightInterval); err != nil {
		return nil, err
	}
	result, err = s.newRankedAlgebraicPolynomialValue(root, reduced, approximation)
	if err != nil {
		return nil, err
	}
	return s.canonicalizeRankedRetainedResult(result, usePrem)
}
