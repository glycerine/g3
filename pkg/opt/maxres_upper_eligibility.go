package opt

// Source: src/opt/maxres.cpp max_correction_set_size upper-block heuristic.

const maxResUpperBlockCorrectionLimit = 3

func (e *maxResEngine) upperBoundEligible(correctionSize int) bool {
	if e == nil || e.objective == nil || correctionSize < 0 {
		return false
	}
	progress := e.stats.Cores+e.stats.CorrectionSets > 0
	limit := int(e.config.maxCorrectionSetSize)
	if limit == 0 {
		limit = maxResUpperBlockCorrectionLimit
	}
	return e.config.addUpperBoundBlock && progress && correctionSize <= limit && len(e.objective.softTerms) != 0
}
