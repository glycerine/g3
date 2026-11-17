package rel

// FetchFunctionalKey looks up a functional row by its non-functional key and
// returns an owned copy of the stored suffix. It is the typed key-only form of
// sparse_table::fetch_fact (the functional columns of the probe are zero)
// and covers the source's zero-column key used by finite-product outer
// indexes.
func (table *SparseTable) FetchFunctionalKey(key Fact) (Fact, bool, error) {
	if err := table.open(); err != nil {
		return nil, false, err
	}
	firstFunctional := table.firstFunctional()
	if table.functional == 0 || len(key) != firstFunctional {
		return nil, false, ErrTable
	}
	probe := make(Fact, table.signature.Len())
	for column, value := range key {
		domain, err := table.signature.Domain(column)
		if err != nil || uint64(value) >= domain {
			return nil, false, ErrTable
		}
		probe[column] = value
	}
	if found, err := table.fetchFunctional(probe); err != nil || !found {
		return nil, false, err
	}
	return append(Fact(nil), probe[firstFunctional:]...), true, nil
}
