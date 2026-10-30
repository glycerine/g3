package rel

func (table *SparseTable) RemoveFacts(facts []Fact) (int, error) {
	if err := table.open(); err != nil {
		return 0, err
	}
	removed := 0
	for _, fact := range facts {
		changed, err := table.Remove(fact)
		if err != nil {
			return removed, err
		}
		if changed {
			removed++
		}
	}
	return removed, nil
}
