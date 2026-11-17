package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/muz/base"
)

func (plan *SparseNegatedJoinPlan) Apply(target, first, second *SparseTable) (_ SparseNegatedJoinStats, resultErr error) {
	stats := SparseNegatedJoinStats{}
	if err := plan.validateTables(target, first, second); err != nil {
		return stats, err
	}
	// negated_join_fn::operator()'s verbose_action _va("negated_join").
	action, err := base.StartVerboseAction("negated_join", base.VerboseActionDefaultLevel)
	if err != nil {
		return stats, err
	}
	defer func() { resultErr = errors.Join(resultErr, action.End()) }()
	targetFacts, err := target.Facts()
	if err != nil {
		return stats, err
	}
	firstFacts, err := first.Facts()
	if err != nil {
		return stats, err
	}
	secondFacts, err := second.Facts()
	if err != nil {
		return stats, err
	}
	stats.TargetRows, stats.FirstRows, stats.SecondRows = len(targetFacts), len(firstFacts), len(secondFacts)
	remove := make([]Fact, 0)
	if sparseNegatedJoinIndexable(plan) {
		if err = plan.collectIndexedRemovals(targetFacts, first, second, &remove, &stats); err != nil {
			return stats, err
		}
		stats.Removed, err = sparseNegatedJoinRemove(target, targetFacts, remove)
		return stats, err
	}
	for _, targetFact := range targetFacts {
		matched := false
		for _, firstFact := range firstFacts {
			stats.FirstProbes++
			if !plan.targetMatchesFirst(targetFact, firstFact) {
				continue
			}
			for _, secondFact := range secondFacts {
				stats.SecondProbes++
				if plan.sourcesJoin(firstFact, secondFact) && plan.targetMatchesSecond(targetFact, secondFact) {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if matched {
			remove = append(remove, targetFact)
		}
	}
	stats.Removed, err = sparseNegatedJoinRemove(target, targetFacts, remove)
	return stats, err
}

// sparseNegatedJoinRemove is negated_join_fn's removal: remove_offset of
// each collected row, then reset_indexes. The source removes the offsets
// in ascending order, which moves the last row into a removed place before
// a later offset is removed (it can then drop the wrong row); g3 removes
// the largest offsets first, as negation_filter_fn does.
func sparseNegatedJoinRemove(target *SparseTable, targetFacts, remove []Fact) (int, error) {
	marked := make(map[string]bool, len(remove))
	for _, f := range remove {
		marked[factKey(f)] = true
	}
	return removeSparseIndexedNegationFacts(target, targetFacts, marked)
}
