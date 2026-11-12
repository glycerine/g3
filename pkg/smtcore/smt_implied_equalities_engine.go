package smtcore

import "errors"

func (e *smtImpliedEqualityEngine) run() (result error) {
	source, modelBased := e.solver.(smtImpliedEqualityEvaluatorSource)
	var evaluator smtImpliedEqualityEvaluator
	if modelBased {
		var err error
		evaluator, err = source.newImpliedEqualityEvaluator()
		if err != nil {
			return err
		}
		modelBased = evaluator != nil
		if modelBased {
			defer func() { result = errors.Join(result, evaluator.Close()) }()
		}
	}
	// get_implied_equalities_impl walks sort2term_ids in table order.
	for _, slot := range e.sortOrder.Entries() {
		sort, indices := slot.Key, e.bySort[slot.Key]
		if !modelBased || e.isArraySort(sort) {
			e.stats.FallbackTerms += uint32(len(indices))
			if err := e.exact(indices); err != nil {
				return err
			}
			continue
		}
		if err := e.modelPartition(indices, evaluator); err != nil {
			return err
		}
	}
	return nil
}

func (e *smtImpliedEqualityEngine) results() []uint32 {
	result := make([]uint32, len(e.terms))
	for index := range result {
		result[index] = e.classes.find(uint32(index))
	}
	return result
}
