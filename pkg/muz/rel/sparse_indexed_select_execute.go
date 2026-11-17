package rel

import "errors"

func (plan *SparseIndexedSelectPlan) Execute(table *SparseTable) (*SparseTable, SparseIndexedSelectStats, error) {
	stats := SparseIndexedSelectStats{}
	if err := plan.validateTable(table); err != nil {
		return nil, stats, err
	}
	rows, err := table.Len()
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = rows
	// select_equal_and_project_fn: res = mk_empty, then t's key indexer
	// on the column and its lookup of the value.
	result, err := plan.newResult(table)
	if err != nil {
		return nil, stats, err
	}
	keyIndexer, err := table.getKeyIndexer([]int{plan.column})
	if err != nil {
		return nil, stats, errors.Join(err, result.Close())
	}
	keyIndexer.lookup(Fact{plan.value})
	indexPlan, err := NewSparseKeyLookupPlan(table, []int{plan.column})
	if err != nil {
		return nil, stats, errors.Join(err, result.Close())
	}
	index, indexStats, err := indexPlan.Build(table)
	if err != nil {
		return nil, stats, errors.Join(err, result.Close())
	}
	stats.IndexGroups, stats.FullSignature = indexStats.Groups, indexStats.FullSignature
	matches, err := index.Lookup(Fact{plan.value})
	if err != nil {
		return nil, stats, errors.Join(err, result.Close())
	}
	stats.Matches = len(matches)
	for _, fact := range matches {
		inserted, addErr := result.Add(sparseProjectFact(fact, []int{plan.column}))
		if addErr != nil {
			_ = result.Close()
			return nil, stats, addErr
		}
		if inserted {
			stats.Inserted++
		}
	}
	return result, stats, nil
}
