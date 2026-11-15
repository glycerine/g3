package opt

// Source: src/opt/maxres.cpp maxres::stats.

import "github.com/glycerine/g3/pkg/util/statistics"

type maxResStatistics struct {
	Cores, Checks, Assumptions, Resolutions       uint32
	Candidates, Improvements, CoreMinChecks       uint32
	CorrectionSets, CorrectionResolutions, Pivots uint32
	UpperBoundBlocks                              uint32
	Mutexes, MutexTerms                           uint32
	Auxiliaries                                   uint32
}

func (e *maxResEngine) statistics() MaxSMTStatistics {
	return MaxSMTStatistics{
		Engine: e.strategy.name(), Checks: e.stats.Checks, Candidates: e.stats.Candidates,
		Improvements: e.stats.Improvements, Cores: e.stats.Cores,
		CoreMinimizationChecks: e.stats.CoreMinChecks, Assumptions: e.stats.Assumptions,
		Resolutions: e.stats.Resolutions, CorrectionSets: e.stats.CorrectionSets,
		CorrectionResolutions: e.stats.CorrectionResolutions, Pivots: e.stats.Pivots,
		UpperBoundBlocks: e.stats.UpperBoundBlocks,
		Mutexes:          e.stats.Mutexes, MutexTerms: e.stats.MutexTerms, Auxiliaries: e.stats.Auxiliaries,
	}
}

// collectStatistics is maxres::collect_statistics.
func (e *maxResEngine) collectStatistics(st *statistics.Statistics) {
	st.UpdateUint("maxres-cores", e.stats.Cores)
	st.UpdateUint("maxres-correction-sets", e.stats.CorrectionSets)
}
