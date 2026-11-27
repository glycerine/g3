package ast

// Source: src/ast/ast_util.cpp:94-158; src/ast/ast_util.h:78-93.

func (m *ASTManager) astUtilOperator(r astNodeRef, kind int32) (bool, error) {
	if e := m.validNode(r); e != nil {
		return false, e
	}
	if m.nodeAt(r.slot).kind != ASTApp {
		return false, nil
	}
	family, op, e := m.appIdentity(m.appAt(r.slot))
	return family == 0 && op == kind, e
}

func (m *ASTManager) astUtilIsAtom(r astNodeRef) (bool, error) {
	if e := m.validNode(r); e != nil {
		return false, e
	}
	if m.nodeAt(r.slot).kind == ASTQuantifier {
		return false, nil
	}
	sort, e := m.sortOf(r)
	if e != nil {
		return false, e
	}
	if sort != m.basic.boolSort {
		return false, nil
	}
	if m.nodeAt(r.slot).kind == ASTVar {
		return true, nil
	}
	if m.nodeAt(r.slot).kind != ASTApp {
		return false, ErrASTDomain
	}
	a := m.appAt(r.slot)
	family, op, e := m.appIdentity(a)
	if e != nil {
		return false, e
	}
	if family != 0 {
		return true, nil
	}
	if op == ASTOpTrue || op == ASTOpFalse {
		return true, nil
	}
	if op != ASTOpEq || a.count == 0 {
		return false, nil
	}
	arg, e := m.appArg(a, 0)
	if e != nil {
		return false, e
	}
	argSort, e := m.sortOf(arg)
	return argSort != m.basic.boolSort, e
}

func (m *ASTManager) astUtilIsLiteral(r astNodeRef) (bool, error) {
	atom, e := m.astUtilIsAtom(r)
	if e != nil || atom {
		return atom, e
	}
	negated, e := m.astUtilOperator(r, ASTOpNot)
	if e != nil || !negated {
		return false, e
	}
	a := m.appAt(r.slot)
	if a.count != 1 {
		return false, ErrASTDomain
	}
	child, e := m.appArg(a, 0)
	if e != nil {
		return false, e
	}
	return m.astUtilIsAtom(child)
}

func (m *ASTManager) astUtilLiteralAtomSign(r astNodeRef) (astNodeRef, bool, error) {
	literal, e := m.astUtilIsLiteral(r)
	if e != nil {
		return astNodeRef{}, false, e
	}
	if !literal {
		return astNodeRef{}, false, ErrASTDomain
	}
	atom, e := m.astUtilIsAtom(r)
	if e != nil || atom {
		return r, false, e
	}
	child, e := m.appArg(m.appAt(r.slot), 0)
	return child, true, e
}

func (m *ASTManager) IsAtom(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.requireExpr(h)
	if e != nil {
		return false, e
	}
	return m.astUtilIsAtom(r)
}

func (m *ASTManager) IsLiteral(h ASTHandle) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	r, e := m.requireExpr(h)
	if e != nil {
		return false, e
	}
	return m.astUtilIsLiteral(r)
}

// LiteralAtomSign returns the literal's atom and whether the literal is
// negated. A nonliteral is rejected as the source assertion precondition.
func (m *ASTManager) LiteralAtomSign(h ASTHandle) (ASTHandle, bool, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, false, e
	}
	defer m.mu.Unlock()
	r, e := m.requireExpr(h)
	if e != nil {
		return ASTHandle{}, false, e
	}
	atom, sign, e := m.astUtilLiteralAtomSign(r)
	return m.external(atom), sign, e
}

func (m *ASTManager) IsClause(h ASTHandle) (bool, error) {
	_, ok, err := m.ClauseLiteralView(h)
	return ok, err
}

func (m *ASTManager) ClauseNumLiterals(h ASTHandle) (uint32, error) {
	literals, ok, err := m.ClauseLiteralView(h)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, ErrASTDomain
	}
	return literals.Count(), nil
}

func (m *ASTManager) ClauseLiteral(h ASTHandle, index uint32) (ASTHandle, error) {
	literals, ok, err := m.ClauseLiteralView(h)
	if err != nil {
		return ASTHandle{}, err
	}
	if !ok {
		return ASTHandle{}, ErrASTDomain
	}
	return literals.At(index)
}

// ClauseLiterals borrows a validated clause. The caller keeps the expression
// alive while reading its literals.
type ClauseLiterals struct {
	manager *ASTManager
	root    ASTHandle
	count   uint32
	unit    bool
}

func (c ClauseLiterals) Count() uint32 { return c.count }

func (c ClauseLiterals) At(index uint32) (ASTHandle, error) {
	if index >= c.count {
		return ASTHandle{}, ErrASTDomain
	}
	if c.unit {
		return c.root, nil
	}
	return c.manager.GetArg(c.root, index)
}

// ClauseLiteralView classifies the expression and checks each disjunct once.
func (m *ASTManager) ClauseLiteralView(h ASTHandle) (ClauseLiterals, bool, error) {
	if err := m.lock(); err != nil {
		return ClauseLiterals{}, false, err
	}
	defer m.mu.Unlock()
	r, err := m.requireExpr(h)
	if err != nil {
		return ClauseLiterals{}, false, err
	}

	literal, err := m.astUtilIsLiteral(r)
	if err != nil {
		return ClauseLiterals{}, false, err
	}
	if literal {
		return ClauseLiterals{manager: m, root: h, count: 1, unit: true}, true, nil
	}
	disjunction, err := m.astUtilOperator(r, ASTOpOr)
	if err != nil || !disjunction {
		return ClauseLiterals{}, false, err
	}
	a := m.appAt(r.slot)
	for i := uint32(0); i < a.count; i++ {
		child, err := m.appArg(a, i)
		if err != nil {
			return ClauseLiterals{}, false, err
		}
		literal, err := m.astUtilIsLiteral(child)
		if err != nil || !literal {
			return ClauseLiterals{}, false, err
		}
	}
	return ClauseLiterals{manager: m, root: h, count: a.count}, true, nil
}
