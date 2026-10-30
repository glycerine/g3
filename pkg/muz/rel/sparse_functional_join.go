package rel

func sparseFunctionalJoinFact(left, right Fact, leftFirst, rightFirst int) Fact {
	result := make(Fact, 0, len(left)+len(right))
	result = append(result, left[:leftFirst]...)
	result = append(result, right[:rightFirst]...)
	result = append(result, left[leftFirst:]...)
	result = append(result, right[rightFirst:]...)
	return result
}

func (left *SparseTable) JoinFunctional(right *SparseTable, leftColumns, rightColumns []int) (*SparseTable, error) {
	if err := left.open(); err != nil {
		return nil, err
	}
	if err := right.open(); err != nil {
		return nil, err
	}
	if err := validateColumns(left.signature, right.signature, leftColumns, rightColumns); err != nil {
		return nil, err
	}
	signature, functional, err := sparseJoinFunctionalSignature(left, right)
	if err != nil {
		return nil, err
	}
	result, err := NewSparseTable(signature, functional)
	if err != nil {
		return nil, err
	}
	leftFacts, _ := left.Facts()
	rightFacts, _ := right.Facts()
	leftFirst := left.signature.Len() - left.functional
	rightFirst := right.signature.Len() - right.functional
	for _, first := range leftFacts {
		for _, second := range rightFacts {
			match := true
			for i := range leftColumns {
				if first[leftColumns[i]] != second[rightColumns[i]] {
					match = false
					break
				}
			}
			if match {
				if _, err = result.Add(sparseFunctionalJoinFact(first, second, leftFirst, rightFirst)); err != nil {
					return nil, err
				}
			}
		}
	}
	return result, nil
}
