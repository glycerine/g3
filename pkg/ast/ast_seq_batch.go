package ast

// Batched sequence recognizers for theory_seq and seq_rewriter hot loops.
// Each answers, under one manager lock, what consecutive single queries
// (SequenceConcatParts, IsSequenceEmpty, SequenceUnitArgument,
// SequenceStringValue, SequenceFlattenConcat) would answer.

// SequenceConcatOrEmpty is SequenceConcatParts(h) followed, when h is not a
// binary concat, by IsSequenceEmpty(h).
func (m *ASTManager) SequenceConcatOrEmpty(h ASTHandle) (left, right ASTHandle, concat, empty bool, err error) {
	if err = m.lock(); err != nil {
		return
	}
	defer m.mu.Unlock()
	r, err := m.requireExpr(h)
	if err != nil {
		return ASTHandle{}, ASTHandle{}, false, false, err
	}
	l, rr, matched, err := m.sequenceBinaryParts(r, ASTSequenceOpConcat)
	if err != nil || matched {
		return m.external(l), m.external(rr), matched, false, err
	}
	empty, err = m.isSequenceEmptyRef(r)
	return ASTHandle{}, ASTHandle{}, false, empty, err
}

// SequenceFlattenConcatAppend is SequenceFlattenConcat(h) appended to dst.
// On error dst is returned unchanged in length.
func (m *ASTManager) SequenceFlattenConcatAppend(h ASTHandle, dst []ASTHandle) ([]ASTHandle, error) {
	if err := m.lock(); err != nil {
		return dst, err
	}
	defer m.mu.Unlock()
	r, err := m.requireExpr(h)
	if err != nil {
		return dst, err
	}
	n := len(dst)
	if err = m.sequenceFlattenConcat(r, &dst); err != nil {
		return dst[:n], err
	}
	return dst, nil
}

// SequenceUnitOrString is SequenceUnitArgument(h) and, when h is not a
// unit, SequenceStringValue(h). A node is at most one of the two.
func (m *ASTManager) SequenceUnitOrString(h ASTHandle) (unit ASTHandle, isUnit bool, value string, isString bool, err error) {
	if err = m.lock(); err != nil {
		return
	}
	defer m.mu.Unlock()
	r, err := m.requireExpr(h)
	if err != nil {
		return ASTHandle{}, false, "", false, err
	}
	p := m.sequencePlugin
	if m.nodeAt(r.slot).kind != ASTApp || p == nil {
		return ASTHandle{}, false, "", false, nil
	}
	a := m.appAt(r.slot)
	family, kind, err := m.appIdentity(a)
	if err != nil || family != p.family {
		return ASTHandle{}, false, "", false, err
	}
	if kind == ASTSequenceOpUnit {
		if a.count != 1 {
			return ASTHandle{}, false, "", false, nil
		}
		argument, err := m.readInternal(a.args, 0)
		return m.external(argument), err == nil, "", false, err
	}
	if kind != ASTSequenceOpStringConst {
		return ASTHandle{}, false, "", false, nil
	}
	value, isString, err = m.sequenceStringConstValue(a.decl)
	return ASTHandle{}, false, value, isString, err
}
