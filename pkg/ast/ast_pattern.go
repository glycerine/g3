package ast

// Source: src/ast/ast.cpp:2312-2327; src/ast/ast.h:1867-1874.

// MkPattern constructs the concrete pattern-plugin application used by
// quantifier triggers.
func (m *ASTManager) MkPattern(exprs []ASTHandle) (ASTHandle, error) {
	if ASTSourceDebug {
		for _, h := range exprs {
			kind, err := m.GetKind(h)
			if err != nil {
				return ASTHandle{}, err
			}
			if kind != ASTApp {
				return ASTHandle{}, ErrASTDomain
			}
		}
	}
	return m.MkFamilyApp(2, 0, nil, exprs)
}

// IsPattern is ast_manager::is_pattern: an OP_PATTERN application of the
// pattern family whose arguments are all applications.
func (m *ASTManager) IsPattern(h ASTHandle) (bool, error) {
	r, err := m.readNode(h)
	if err != nil {
		return false, err
	}
	return m.isPatternRef(r)
}

// isPatternRef is ast_manager::is_pattern (ast.cpp:2320-2331) for a live
// node reference.
func (m *ASTManager) isPatternRef(r astNodeRef) (bool, error) {
	if m.nodeAt(r.slot).kind != ASTApp {
		return false, nil
	}
	a := m.appAt(r.slot)
	family, op, err := m.appIdentity(a)
	if err != nil || family != 2 || op != 0 {
		return false, err
	}
	for i := uint32(0); i < a.count; i++ {
		arg, err := m.appArg(a, i)
		if err != nil {
			return false, err
		}
		if err = m.checkNode(arg); err != nil {
			return false, err
		}
		if m.nodeAt(arg.slot).kind != ASTApp {
			return false, nil
		}
	}
	return true, nil
}
