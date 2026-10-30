package rel

func (table *SparseTable) ContainsExact(fact Fact) (bool, error) {
	if err := table.open(); err != nil {
		return false, err
	}
	if _, err := table.layout.pack(table.signature, fact); err != nil {
		return false, err
	}
	facts, err := table.Facts()
	if err != nil {
		return false, err
	}
	for _, stored := range facts {
		if len(stored) != len(fact) {
			continue
		}
		equal := true
		for column := range stored {
			if stored[column] != fact[column] {
				equal = false
				break
			}
		}
		if equal {
			return true, nil
		}
	}
	return false, nil
}

func (table *SparseTable) FetchFact(fact Fact) (bool, error) {
	if err := table.open(); err != nil {
		return false, err
	}
	packed, err := table.layout.pack(table.signature, fact)
	if err != nil {
		return false, err
	}
	index, found := table.data.find(packed)
	if !found {
		return false, nil
	}
	stored := table.layout.unpack(table.data.entries[index])
	copy(fact[table.signature.Len()-table.functional:], stored[table.signature.Len()-table.functional:])
	return true, nil
}

func (table *SparseTable) SuggestFact(fact Fact) (bool, error) {
	if found, err := table.FetchFact(fact); err != nil || found {
		return false, err
	}
	return table.Add(fact)
}

func (table *SparseTable) EnsureFact(fact Fact) error {
	if err := table.open(); err != nil {
		return err
	}
	if table.functional == 0 {
		_, err := table.Add(fact)
		return err
	}
	if _, err := table.layout.pack(table.signature, fact); err != nil {
		return err
	}
	table.Remove(fact)
	_, err := table.Add(fact)
	return err
}
