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

// SequenceFlattenConcatAppendHeld is SequenceFlattenConcatAppend(h, dst)
// followed by IncRefEach on the appended leaves, under one manager lock.
// A flatten failure returns dst unchanged in length (flattenErr); a
// reference failure keeps only the leaves referenced before it (refErr).
func (m *ASTManager) SequenceFlattenConcatAppendHeld(h ASTHandle, dst []ASTHandle) (out []ASTHandle, flattenErr, refErr error) {
	if flattenErr = m.lock(); flattenErr != nil {
		return dst, flattenErr, nil
	}
	defer m.mu.Unlock()
	r, err := m.requireExpr(h)
	if err != nil {
		return dst, err, nil
	}
	n := len(dst)
	if err = m.sequenceFlattenConcat(r, &dst); err != nil {
		return dst[:n], err, nil
	}
	if err = m.relockBetween(); err != nil {
		return dst[:n], nil, err
	}
	for i := n; i < len(dst); i++ {
		if dst[i].IsNull() {
			continue
		}
		r, e := m.internal(dst[i])
		if e == nil {
			e = m.incNode(r)
		}
		if e != nil {
			return dst[:i], nil, e
		}
	}
	return dst, nil, nil
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
	return m.sequenceUnitOrStringRef(r)
}

func (m *ASTManager) sequenceUnitOrStringRef(r astNodeRef) (unit ASTHandle, isUnit bool, value string, isString bool, err error) {
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
		argument, err := m.appArg(a, 0)
		return m.external(argument), err == nil, "", false, err
	}
	if kind != ASTSequenceOpStringConst {
		return ASTHandle{}, false, "", false, nil
	}
	value, isString, err = m.sequenceStringConstValue(a.decl)
	return ASTHandle{}, false, value, isString, err
}

// SequenceEqEnd describes one end element of a sequence equation side, as
// seq_rewriter::reduce_eq inspects it.
type SequenceEqEnd struct {
	Empty    bool
	Unit     ASTHandle
	IsUnit   bool
	Str      string
	IsString bool
}

// SequenceEqEnds answers, under one manager lock, the queries reduce_eq
// makes about the end elements l and r of its two sides (a null handle is
// an exhausted side): IsSequenceEmpty of both, in the order leftFirst
// selects; when neither is empty and both are present, SequenceUnitOrString
// of r and then of l; and when l != r and both are units, AreDistinct of
// their arguments (distinct). Every query is a pure read, so answering
// queries the caller then skips changes nothing. The first failing query's
// error is returned.
func (m *ASTManager) SequenceEqEnds(l, r ASTHandle, leftFirst bool) (le, re SequenceEqEnd, distinct bool, err error) {
	if err = m.lock(); err != nil {
		return
	}
	defer m.mu.Unlock()
	var lr, rr astNodeRef
	empty := func(h ASTHandle, x *astNodeRef, end *SequenceEqEnd) error {
		if h.IsNull() {
			return nil
		}
		var e error
		if *x, e = m.requireExpr(h); e != nil {
			return e
		}
		end.Empty, e = m.isSequenceEmptyRef(*x)
		return e
	}
	if leftFirst {
		if err = empty(l, &lr, &le); err == nil {
			err = empty(r, &rr, &re)
		}
	} else if err = empty(r, &rr, &re); err == nil {
		err = empty(l, &lr, &le)
	}
	if err != nil || le.Empty || re.Empty || l.IsNull() || r.IsNull() {
		return
	}
	if re.Unit, re.IsUnit, re.Str, re.IsString, err = m.sequenceUnitOrStringRef(rr); err != nil {
		return
	}
	if le.Unit, le.IsUnit, le.Str, le.IsString, err = m.sequenceUnitOrStringRef(lr); err != nil {
		return
	}
	if l != r && le.IsUnit && re.IsUnit {
		distinct, err = m.areDistinctLocked(le.Unit, re.Unit)
	}
	return
}

// IncRefSequenceConcatOrEmpty is IncRef(h) followed by
// SequenceConcatOrEmpty(h), under one manager lock. A failed IncRef
// returns its error with held false and no shape.
func (m *ASTManager) IncRefSequenceConcatOrEmpty(h ASTHandle) (left, right ASTHandle, concat, empty, held bool, err error) {
	if err = m.lock(); err != nil {
		return
	}
	defer m.mu.Unlock()
	if !h.IsNull() {
		r, e := m.internal(h)
		if e == nil {
			e = m.incNode(r)
		}
		if e != nil {
			return ASTHandle{}, ASTHandle{}, false, false, false, e
		}
	}
	held = true
	r, err := m.requireExpr(h)
	if err != nil {
		return ASTHandle{}, ASTHandle{}, false, false, held, err
	}
	l, rr, matched, err := m.sequenceBinaryParts(r, ASTSequenceOpConcat)
	if err != nil || matched {
		return m.external(l), m.external(rr), matched, false, held, err
	}
	empty, err = m.isSequenceEmptyRef(r)
	return ASTHandle{}, ASTHandle{}, false, empty, held, err
}
