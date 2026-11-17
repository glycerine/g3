package rel

// removeSparseIndexedNegationFacts is negation_filter_fn's removal: the
// marked rows' offsets, largest first, go to remove_offset, then
// reset_indexes. targetFacts are the target's rows in storage order.
func removeSparseIndexedNegationFacts(target *SparseTable, targetFacts []Fact, marked map[string]bool) (int, error) {
	if err := target.open(); err != nil {
		return 0, err
	}
	removed := 0
	for i := len(targetFacts) - 1; i >= 0; i-- {
		if !marked[factKey(targetFacts[i])] {
			continue
		}
		target.data.removeOffset(storeOffset(i) * storeOffset(target.factSize))
		removed++
	}
	target.resetIndexes()
	return removed, nil
}
