package ast

// SequenceExprShape answers in one call what theory_seq's is_var asks with
// separate recognizers: whether h has a sequence sort (is_seq), whether it
// is an application, and that application's declaration family and kind.
// sequenceFamily is the sequence plugin's family, or NullFamilyID without
// one. Kind and family are only read when h is a sequence application.
func (m *ASTManager) SequenceExprShape(h ASTHandle) (isSeq, isApp bool, family FamilyID, kind int32, sequenceFamily FamilyID, err error) {
	sequenceFamily = NullFamilyID
	if err = m.lock(); err != nil {
		return
	}
	defer m.mu.Unlock()
	r, err := m.requireExpr(h)
	if err != nil {
		return
	}
	sort, err := m.sortOf(r)
	if err != nil {
		return
	}
	if _, isSeq, err = m.sequenceSortParameter(sort, 0); err != nil || !isSeq {
		return false, false, 0, 0, sequenceFamily, err
	}
	if p := m.sequencePlugin; p != nil {
		sequenceFamily = p.family
	}
	if m.nodeAt(r.slot).kind != ASTApp {
		return true, false, 0, 0, sequenceFamily, nil
	}
	family, kind, err = m.declIdentity(m.appAt(r.slot).decl)
	return true, true, family, kind, sequenceFamily, err
}

// IsSequenceExpr is GetSort followed by SequenceSortElement's match flag in
// one call: whether expression h has a sequence sort.
func (m *ASTManager) IsSequenceExpr(h ASTHandle) (bool, error) {
	if err := m.lock(); err != nil {
		return false, err
	}
	defer m.mu.Unlock()
	r, err := m.internal(h)
	if err != nil {
		return false, err
	}
	sort, err := m.sortOf(r)
	if err != nil {
		return false, err
	}
	_, isSeq, err := m.sequenceSortParameter(sort, 0)
	return isSeq, err
}

// IncRefEach is IncRef on each handle in order under one manager lock. It
// stops at the first failure and reports how many references it took.
func (m *ASTManager) IncRefEach(hs []ASTHandle) (int, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	for i, h := range hs {
		if h.IsNull() {
			continue
		}
		r, e := m.internal(h)
		if e == nil {
			e = m.incNode(r)
		}
		if e != nil {
			return i, e
		}
	}
	return len(hs), nil
}
