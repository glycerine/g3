package smtcore

// testClauses lists m_aux_clauses, then m_lemmas, then the watch-only
// binary clauses. The records are copies; their clause objects are shared.
func (s *SMTInternalizerState) testClauses() []SMTInternalizedClause {
	result := make([]SMTInternalizedClause, 0, s.numClauseRecords())
	for _, records := range s.clauseRecordLists() {
		result = append(result, records...)
	}
	return s.binaryClauses.appendSnapshots(result)
}
