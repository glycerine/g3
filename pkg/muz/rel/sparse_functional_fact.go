package rel

func (table *SparseTable) ContainsExact(fact Fact) (bool, error) {
	return table.Contains(fact)
}

// FetchFact is sparse_table::fetch_fact without its verbose_action: the
// functional columns of the stored row with fact's key are copied into fact.
func (table *SparseTable) FetchFact(fact Fact) (bool, error) {
	if err := table.open(); err != nil {
		return false, err
	}
	if err := table.layout.checkFact(table.signature, fact); err != nil {
		return false, err
	}
	if table.functional == 0 {
		return table.Contains(fact)
	}
	table.writeIntoReserve(fact)
	ofs, found := table.data.findReserveContent()
	if !found {
		return false, nil
	}
	for i := table.firstFunctional(); i < table.signature.Len(); i++ {
		fact[i] = table.getCell(ofs, i)
	}
	return true, nil
}

func (table *SparseTable) SuggestFact(fact Fact) (bool, error) {
	if found, err := table.FetchFact(fact); err != nil || found {
		return false, err
	}
	return table.Add(fact)
}

// EnsureFact is sparse_table::ensure_fact without its verbose_action: the
// functional columns of an existing row are overwritten in place.
func (table *SparseTable) EnsureFact(fact Fact) error {
	if err := table.open(); err != nil {
		return err
	}
	if table.functional == 0 {
		_, err := table.Add(fact)
		return err
	}
	if err := table.layout.checkFact(table.signature, fact); err != nil {
		return err
	}
	table.writeIntoReserve(fact)
	ofs, found := table.data.findReserveContent()
	if !found {
		_, err := table.Add(fact)
		return err
	}
	for i := table.firstFunctional(); i < table.signature.Len(); i++ {
		table.setCell(ofs, i, fact[i])
	}
	return nil
}
