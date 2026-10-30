package rel

func (table *SparseTable) ProjectFunctional(removed []int) (*SparseTable, error) {
	if err := table.open(); err != nil {
		return nil, err
	}
	signature, err := projectedSignature(table.signature, removed)
	if err != nil {
		return nil, err
	}
	functional, err := sparseProjectFunctionalCount(table, removed)
	if err != nil {
		return nil, err
	}
	result, err := NewSparseTable(signature, functional)
	if err != nil {
		return nil, err
	}
	facts, err := table.Facts()
	if err != nil {
		return nil, err
	}
	for _, fact := range facts {
		if _, err = result.Add(sparseProjectFact(fact, removed)); err != nil {
			return nil, err
		}
	}
	return result, nil
}
