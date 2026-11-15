package sat

// Source: src/sat/sat_solver.cpp:844-856,876-880.

import "github.com/glycerine/g3/pkg/util/lbool"

// transfer is the first-finisher branch of check_par: a winning worker's
// model or core is copied into the owner immediately.
func (p *satParallelPortfolio) transfer(winner *Solver, r lbool.LBool) error {
	switch r {
	case lbool.True:
		if err := p.owner.setModel(&winner.model); err != nil {
			return err
		}
	case lbool.False:
		if err := p.owner.conflict.core.Shrink(0); err != nil {
			return err
		}
		if err := satMUSAppend(&p.owner.conflict.core, &winner.conflict.core); err != nil {
			return err
		}
	}
	return nil
}

// adopt copies the winning worker's statistics after all lanes ran
// (sat_solver.cpp:878-880).
func (p *satParallelPortfolio) adopt() error {
	if p.finished < 0 || p.finished >= len(p.workers) {
		return nil
	}
	p.owner.copyParallelMainStatistics(p.workers[p.finished])
	return nil
}

func (s *Solver) copyParallelMainStatistics(src *Solver) {
	s.Variables.mkVarCount = src.Variables.mkVarCount
	s.Lifecycle.mkBinary = src.Lifecycle.mkBinary
	s.Lifecycle.mkTernary = src.Lifecycle.mkTernary
	s.Lifecycle.mkNary = src.Lifecycle.mkNary
	s.gc.gcClauseStat = src.gc.gcClauseStat
	s.Lifecycle.deleted = src.Lifecycle.deleted
	s.conflict.conflictStat = src.conflict.conflictStat
	s.propagation.clausePropagations = src.propagation.clausePropagations
	s.decision.decisions = src.decision.decisions
	s.propagation.binaryPropagations = src.propagation.binaryPropagations
	s.propagation.ternaryPropagations = src.propagation.ternaryPropagations
	s.gc.restartStat = src.gc.restartStat
	s.analysis.minimizedLits = src.analysis.minimizedLits
	s.dynamic.eliminated = src.dynamic.eliminated
	s.blockedCorrectionSets = src.blockedCorrectionSets
}
