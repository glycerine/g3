package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:336-347.

func (s *subsumptionSimplifier) cleanupRemoved() error {
	return s.solver.deleteRemovedClauses(s.removed)
}

// deleteRemovedClauses deletes the marked clauses in clause-vector order, as
// the source cleanup_removed walks m_clauses.
func (s *Solver) deleteRemovedClauses(removed map[*Clause]bool) error {
	if len(removed) == 0 {
		return nil
	}
	ordered := make([]*Clause, 0, len(removed))
	for _, list := range [][]*Clause{s.clauses.originals, s.clauses.learned} {
		for _, clause := range list {
			if removed[clause] {
				ordered = append(ordered, clause)
			}
		}
	}
	if len(ordered) != len(removed) {
		return ErrAtomCacheOwner // a marked clause is not in the clause store
	}
	for _, clause := range ordered {
		if err := s.clauses.deleteClause(clause); err != nil {
			return err
		}
	}
	return nil
}
