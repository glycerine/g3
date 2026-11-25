package smtcore

// testClauses lists m_aux_clauses, then m_lemmas, then the watch-only
// binary clauses. The records are copies; their clause objects are shared.
func (s *SMTInternalizerState) testClauses() []SMTInternalizedClause {
	result := make([]SMTInternalizedClause, 0, s.numClauseRecords())
	for _, records := range s.clauseRecordLists() {
		for i := range records {
			result = append(result, records[i].snapshot())
		}
	}
	return s.binaryClauses.appendSnapshots(result)
}

func (c *SMTClause) snapshot() SMTInternalizedClause {
	out := SMTInternalizedClause{clause: c.activeClause(), Root: c.directoryFlags&smtRecordRoot != 0, Literals: c.directoryLiterals(), closeWatches: c.watchProgress()}
	if d := c.directoryDetails(); d != nil {
		out.Proof = d.proof
		out.owner = d.owner
	}
	return out
}
