package ast

// Source: src/ast/ast.h; src/ast/rewriter/rewriter_def.h.

func (m *ASTManager) RewriterMakeApp(d ASTHandle, args AppArguments) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	decl, e := m.requireKind(d, ASTFuncDecl)
	if e != nil {
		return ASTHandle{}, e
	}
	h, e := m.mkAppArguments(decl, args)
	return m.external(h), e
}

func (m *ASTManager) RewriterAssociative(d ASTHandle) (bool, error) {
	decl, e := m.readKind(d, ASTFuncDecl)
	if e != nil {
		return false, e
	}
	flags, e := m.funcFlags(decl)
	return flags.LeftAssociative && flags.RightAssociative, e
}

func (m *ASTManager) RewriterCommutative(d ASTHandle) (bool, error) {
	decl, e := m.readKind(d, ASTFuncDecl)
	if e != nil {
		return false, e
	}
	flags, e := m.funcFlags(decl)
	return flags.Commutative, e
}

// RewriterPattern is m_manager.is_pattern(np[i]) in
// rewriter_tpl::reduce_quantifier.
func (m *ASTManager) RewriterPattern(t ASTHandle) (bool, error) {
	return m.IsPattern(t)
}

// AppArgumentView borrows an application's checked handle array for a source
// callback. The application must remain alive until the view is discarded.
func (m *ASTManager) AppArgumentView(app ASTHandle) (AppArguments, error) {
	r, e := m.readKind(app, ASTApp)
	if e != nil {
		return AppArguments{}, e
	}
	return AppArguments{app: app, appOwner: m, count: m.appAt(r.slot).count}, nil
}

// SubstitutionShape reads an expression's fixed shape and current reference count.
// arityOrIndex is the arity of an application or the de Bruijn index of a variable.
func (m *ASTManager) SubstitutionShape(h ASTHandle) (kind ASTKind, refs, arityOrIndex uint32, ground bool, err error) {
	if !m.fastHandle(h) {
		if _, err = m.readExpr(h); err != nil {
			return
		}
	}
	slot := &m.nodes[(h.slot-1)/astPageSize].slots[(h.slot-1)%astPageSize]
	switch slot.node.kind {
	case ASTApp:
		return ASTApp, slot.node.rc, slot.app.count, slot.app.ground, nil
	case ASTVar:
		i := slot.node.kindIndex - 1
		return ASTVar, slot.node.rc, m.vars[i/astPageSize][i%astPageSize].index, false, nil
	case ASTQuantifier:
		return ASTQuantifier, slot.node.rc, 0, false, nil
	}
	return 0, 0, 0, false, ErrASTDomain
}
