package rel

func (plan *SparseIndexedJoinPlan) executeIndexed(left, right *SparseTable, leftFacts, rightFacts []Fact, result *SparseTable, stats *SparseIndexedJoinStats) error {
	stats.IndexedLeft = len(leftFacts) > len(rightFacts)
	indexedTable, indexedColumns := right, plan.rightColumns
	probeFacts, probeColumns := leftFacts, plan.leftColumns
	if stats.IndexedLeft {
		indexedTable, indexedColumns = left, plan.leftColumns
		probeFacts, probeColumns = rightFacts, plan.rightColumns
	}
	// self_agnostic_join_project: the indexed table's get_key_indexer; the
	// lookup runs for the first probe row and whenever its key changes.
	keyIndexer, err := indexedTable.getKeyIndexer(indexedColumns)
	if err != nil {
		return err
	}
	tracker := newSparseKeyTracker(len(indexedColumns))
	indexPlan, err := NewSparseKeyLookupPlan(indexedTable, indexedColumns)
	if err != nil {
		return err
	}
	index, indexStats, err := indexPlan.Build(indexedTable)
	if err != nil {
		return err
	}
	stats.IndexGroups = indexStats.Groups
	for _, probe := range probeFacts {
		stats.Probes++
		key := sparseIndexedJoinKey(probe, probeColumns)
		if tracker.update(key) {
			keyIndexer.lookup(key)
		}
		matches, err := index.Lookup(key)
		if err != nil {
			return err
		}
		for _, match := range matches {
			leftFact, rightFact := probe, match
			if stats.IndexedLeft {
				leftFact, rightFact = match, probe
			}
			if err := plan.addResult(result, sparseIndexedJoinedFact(left, right, leftFact, rightFact), stats); err != nil {
				return err
			}
		}
	}
	return nil
}
