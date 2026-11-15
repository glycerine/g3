package opt

// Source: src/opt/opt_context.cpp context::collect_statistics (the m_solver
// and m_maxsmts parts) and src/opt/maxsmt.cpp maxsmt::collect_statistics.

import (
	"github.com/glycerine/g3/pkg/util/statistics"
)

// CollectSolverStatistics is m_solver->collect_statistics(st) while m_solver
// is this run's opt_solver (or a still-attached inc_sat_solver).
func (s *ArithmeticSolver) CollectSolverStatistics(st *statistics.Statistics) error {
	if s == nil || st == nil {
		return nil
	}
	if s.sat != nil {
		return s.sat.CollectStatistics(st)
	}
	if s.backend == nil {
		return nil
	}
	return s.backend.CollectStatistics(st)
}

// CollectMaxSMTStatistics walks m_maxsmts: each maxsmt forwards to its
// m_msolver, the last engine run for that objective. Only maxres reports
// counters; sortmax and wmax collect nothing.
func (s *ArithmeticSolver) CollectMaxSMTStatistics(st *statistics.Statistics) {
	if s == nil || st == nil {
		return
	}
	for i := range s.softEngines {
		superseded := false
		for j := i + 1; j < len(s.softEngines); j++ {
			if s.softEngines[j].index == s.softEngines[i].index {
				superseded = true
				break
			}
		}
		if superseded {
			continue
		}
		if engine, ok := s.softEngines[i].engine.(*maxResEngine); ok {
			engine.collectStatistics(st)
		}
	}
}
