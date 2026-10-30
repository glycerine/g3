package rel

func (target *SparseTable) FilterNegationFunctional(negative *SparseTable, targetColumns, negativeColumns []int) (int, error) {
	if err := target.open(); err != nil {
		return 0, err
	}
	if err := negative.open(); err != nil {
		return 0, err
	}
	if err := validateColumns(target.signature, negative.signature, targetColumns, negativeColumns); err != nil {
		return 0, err
	}
	if sparseJoinInvolvesFunctional(target, negative, targetColumns, negativeColumns) {
		return 0, ErrTable
	}
	targetFacts, _ := target.Facts()
	negativeFacts, _ := negative.Facts()
	removed := 0
	for _, fact := range targetFacts {
		matched := false
		for _, excluded := range negativeFacts {
			match := true
			for i := range targetColumns {
				if fact[targetColumns[i]] != excluded[negativeColumns[i]] {
					match = false
					break
				}
			}
			if match {
				matched = true
				break
			}
		}
		if matched {
			changed, err := target.Remove(fact)
			if err != nil {
				return removed, err
			}
			if changed {
				removed++
			}
		}
	}
	return removed, nil
}
