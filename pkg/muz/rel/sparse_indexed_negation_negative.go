package rel

func (plan *SparseIndexedNegationPlan) collectWithNegativeOuter(target *SparseTable, negativeFacts []Fact, marked map[string]bool, stats *SparseIndexedNegationStats) error {
	// collect_intersection_offsets: t2's get_key_indexer; the lookup runs
	// for the first t1 row and whenever its key changes.
	keyIndexer, err := target.getKeyIndexer(plan.targetColumns)
	if err != nil {
		return err
	}
	tracker := newSparseKeyTracker(len(plan.targetColumns))
	indexPlan, err := NewSparseKeyLookupPlan(target, plan.targetColumns)
	if err != nil {
		return err
	}
	index, indexStats, err := indexPlan.Build(target)
	if err != nil {
		return err
	}
	stats.IndexGroups = indexStats.Groups
	for _, negativeFact := range negativeFacts {
		stats.Probes++
		key := sparseIndexedJoinKey(negativeFact, plan.negativeColumns)
		if tracker.update(key) {
			keyIndexer.lookup(key)
		}
		matches, err := index.Lookup(key)
		if err != nil {
			return err
		}
		stats.Candidates += len(matches)
		for _, targetFact := range matches {
			marked[factKey(targetFact)] = true
		}
	}
	return nil
}
