package rel

// RemoveFunctionalKey removes the row identified by its nonfunctional prefix.
// sparse_table::remove_fact hashes only this unique prefix even though its
// public entry point receives a complete row (the functional columns of
// the reserve are written as zero here).
func (table *SparseTable) RemoveFunctionalKey(key Fact) (bool, error) {
	if err := table.open(); err != nil {
		return false, err
	}
	if table.functional <= 0 {
		return false, ErrTable
	}
	firstFunctional := table.firstFunctional()
	if len(key) != firstFunctional {
		return false, ErrTable
	}
	probe := make(Fact, table.signature.Len())
	for column, value := range key {
		if uint64(value) >= table.signature.domains[column] {
			return false, ErrTable
		}
		probe[column] = value
	}
	return table.removeFact(probe), nil
}
