package rel

func sparseJoinedColumn(table *SparseTable, column, nonFunctionalOffset, functionalOffset int) int {
	first := table.signature.Len() - table.functional
	if column < first {
		return nonFunctionalOffset + column
	}
	return functionalOffset + column - first
}

func sparseJoinProjectFunctionalCount(left, right *SparseTable, leftColumns, rightColumns, removed []int) (int, error) {
	joinedSignature, joinedFunctional, err := sparseJoinFunctionalSignature(left, right)
	if err != nil {
		return 0, err
	}
	if _, err = projectedSignature(joinedSignature, removed); err != nil {
		return 0, err
	}
	leftFirst := left.signature.Len() - left.functional
	rightFirst := right.signature.Len() - right.functional
	firstFunctional := leftFirst + rightFirst
	equivalence := newSparseColumnEquivalence(joinedSignature.Len())
	for i := range leftColumns {
		leftIndex := sparseJoinedColumn(left, leftColumns[i], 0, firstFunctional)
		rightIndex := sparseJoinedColumn(right, rightColumns[i], leftFirst, firstFunctional+left.functional)
		equivalence.merge(leftIndex, rightIndex)
	}
	remaining := make([]int, joinedSignature.Len())
	for column := 0; column < firstFunctional; column++ {
		remaining[equivalence.find(column)]++
	}
	mergesRows := false
	removedFunctional := 0
	for _, column := range removed {
		if column >= firstFunctional {
			removedFunctional++
			continue
		}
		root := equivalence.find(column)
		if remaining[root] > 1 {
			remaining[root]--
		} else {
			mergesRows = true
			break
		}
	}
	if mergesRows {
		return 0, nil
	}
	return joinedFunctional - removedFunctional, nil
}
