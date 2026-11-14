package ast

// Unchecked hot AST reads; astValidateHandles (ast_validate_handles_*.go)
// selects whether they validate.

// readNode is lock() followed by internal(h) for a read accessor. In normal
// builds it only checks for a null handle and pending node deletions (both
// take the checked path, which reports exactly what it did before).
func (m *ASTManager) readNode(h ASTHandle) (astNodeRef, error) {
	if astValidateHandles || h.slot == 0 || len(m.pendingDeletions) != 0 {
		return m.readNodeChecked(h)
	}
	return astNodeRef{h.slot, h.generation}, nil
}
func (m *ASTManager) readNodeChecked(h ASTHandle) (astNodeRef, error) {
	if e := m.lock(); e != nil {
		return astNodeRef{}, e
	}
	return m.internal(h)
}

// readKind is lock() followed by requireKind(h, kind).
func (m *ASTManager) readKind(h ASTHandle, kind ASTKind) (astNodeRef, error) {
	if astValidateHandles || h.slot == 0 || len(m.pendingDeletions) != 0 {
		if e := m.lock(); e != nil {
			return astNodeRef{}, e
		}
		return m.requireKind(h, kind)
	}
	if m.nodeAt(h.slot).kind != kind {
		return astNodeRef{}, ErrASTDomain
	}
	return astNodeRef{h.slot, h.generation}, nil
}

// readExpr is lock() followed by requireExpr(h).
func (m *ASTManager) readExpr(h ASTHandle) (astNodeRef, error) {
	if astValidateHandles || h.slot == 0 || len(m.pendingDeletions) != 0 {
		if e := m.lock(); e != nil {
			return astNodeRef{}, e
		}
		return m.requireExpr(h)
	}
	if m.nodeAt(h.slot).kind > ASTQuantifier {
		return astNodeRef{}, ErrASTDomain
	}
	return astNodeRef{h.slot, h.generation}, nil
}

// checkNode is validNode(r) for a reference read from a live record; normal
// builds check only for the null reference.
func (m *ASTManager) checkNode(r astNodeRef) error {
	if astValidateHandles || r.slot == 0 {
		return m.validNode(r)
	}
	return nil
}

// ASTValidateHandles reports whether this build validates handles on every
// AST read (g3checked or g3debug). Tests that pass stale, foreign or null
// handles, or use a closed manager, through read accessors depend on it.
const ASTValidateHandles = astValidateHandles

// IsLiveHandle reports whether h names a live node of m. It always
// validates, in every build; tests use it to probe that a node was deleted.
func (m *ASTManager) IsLiveHandle(h ASTHandle) bool {
	if m.lock() != nil {
		return false
	}
	_, e := m.internal(h)
	return e == nil
}
