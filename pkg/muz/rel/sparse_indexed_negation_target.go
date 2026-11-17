package rel

func (plan *SparseIndexedNegationPlan) collectWithTargetOuter(negative *SparseTable, targetFacts []Fact, marked map[string]bool, stats *SparseIndexedNegationStats) error {
	// collect_intersection_offsets: t2's get_key_indexer; the lookup runs
	// for the first t1 row and whenever its key changes.
	keyIndexer, err := negative.getKeyIndexer(plan.negativeColumns)
	if err != nil {
		return err
	}
	tracker := newSparseKeyTracker(len(plan.negativeColumns))
	indexPlan, err := NewSparseKeyLookupPlan(negative, plan.negativeColumns)
	if err != nil {
		return err
	}
	index, indexStats, err := indexPlan.Build(negative)
	if err != nil {
		return err
	}
	stats.IndexGroups = indexStats.Groups
	for _, targetFact := range targetFacts {
		stats.Probes++
		key := sparseIndexedJoinKey(targetFact, plan.targetColumns)
		if tracker.update(key) {
			keyIndexer.lookup(key)
		}
		matches, err := index.Lookup(key)
		if err != nil {
			return err
		}
		stats.Candidates += len(matches)
		if len(matches) != 0 {
			marked[factKey(targetFact)] = true
		}
	}
	return nil
}
