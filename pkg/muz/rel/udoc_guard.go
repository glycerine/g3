package rel

import "github.com/glycerine/g3/pkg/ast"

// IsGuard recognizes the Boolean and finite bit-vector forms consumed by the
// UDoc guard compiler in src/muz/rel/udoc_relation.cpp.
func (r *UDocRelation) IsGuard(manager *ast.ASTManager, expression ast.ASTHandle) (bool, error) {
	if err := r.check(); err != nil {
		return false, err
	}
	if manager == nil || expression == (ast.ASTHandle{}) {
		return false, ErrDOC
	}
	return udocIsGuard(manager, expression)
}

// ExtractGuard splits a conjunction into the source's directly compilable
// guards and the residual expressions handled by the theory rewriter.
func (r *UDocRelation) ExtractGuard(manager *ast.ASTManager, condition ast.ASTHandle) (guard, rest ast.ASTHandle, err error) {
	if err = r.check(); err != nil {
		return
	}
	if manager == nil || condition == (ast.ASTHandle{}) {
		err = ErrDOC
		return
	}
	// extract_guard: conds = flatten_and([cond]) (ast_util's worklist, with
	// its swap-with-back order and negation pushing).
	terms, err := manager.FlattenAnd([]ast.ASTHandle{condition})
	if err != nil {
		return
	}
	guards := make([]ast.ASTHandle, 0, len(terms))
	remainders := make([]ast.ASTHandle, 0, len(terms))
	for _, term := range terms {
		var isGuard bool
		isGuard, err = udocIsGuard(manager, term)
		if err != nil {
			return
		}
		if isGuard {
			guards = append(guards, term)
		} else {
			remainders = append(remainders, term)
		}
	}
	guard, err = udocMakeAnd(manager, guards)
	if err != nil {
		return
	}
	rest, err = udocMakeAnd(manager, remainders)
	return
}

func udocMakeAnd(manager *ast.ASTManager, terms []ast.ASTHandle) (ast.ASTHandle, error) {
	switch len(terms) {
	case 0:
		return manager.MkTrue()
	case 1:
		return terms[0], nil
	default:
		return manager.MkAndArgs(terms)
	}
}

func udocIsGuard(m *ast.ASTManager, h ast.ASTHandle) (bool, error) {
	if m.IsTrue(h) || m.IsFalse(h) {
		return true, nil
	}
	kind, err := m.GetKind(h)
	if err != nil {
		return false, err
	}
	if kind == ast.ASTVar {
		return true, nil
	}
	if kind != ast.ASTApp {
		return false, nil
	}
	family, err := m.GetFamilyID(h)
	if err != nil {
		return false, err
	}
	declKind, err := m.GetDeclKind(h)
	if err != nil {
		return false, err
	}
	if family == m.BasicFamilyID() {
		switch declKind {
		case ast.ASTOpAnd, ast.ASTOpOr, ast.ASTOpNot:
			n, e := m.GetNumArgs(h)
			if e != nil {
				return false, e
			}
			for i := uint32(0); i < n; i++ {
				a, e := m.GetArg(h, i)
				if e != nil {
					return false, e
				}
				ok, e := udocIsGuard(m, a)
				if e != nil || !ok {
					return ok, e
				}
			}
			return true, nil
		case ast.ASTOpEq:
			a, e := m.GetArg(h, 0)
			if e != nil {
				return false, e
			}
			b, e := m.GetArg(h, 1)
			if e != nil {
				return false, e
			}
			return udocGuardEquality(m, a, b)
		}
	}
	return false, nil
}

func udocGuardEquality(m *ast.ASTManager, a, b ast.ASTHandle) (bool, error) {
	if ok, err := udocVarRange(m, a); err != nil || ok {
		ground, e := m.IsGround(b)
		return ground && ok, joinGuardErrors(err, e)
	}
	if ok, err := udocVarRange(m, b); err != nil || ok {
		ground, e := m.IsGround(a)
		return ground && ok, joinGuardErrors(err, e)
	}
	return false, nil
}

func udocVarRange(m *ast.ASTManager, h ast.ASTHandle) (bool, error) {
	if kind, err := m.GetKind(h); err != nil {
		return false, err
	} else if kind == ast.ASTVar {
		sort, e := m.GetSort(h)
		if e != nil {
			return false, e
		}
		_, e = m.BitVectorSize(sort)
		return e == nil, nil
	}
	var low, high uint32
	var base ast.ASTHandle
	found, err := m.BitVectorExtractValue(h, &low, &high, &base)
	if err != nil || !found {
		return false, err
	}
	kind, err := m.GetKind(base)
	if err != nil || kind != ast.ASTVar {
		return false, err
	}
	sort, err := m.GetSort(base)
	if err != nil {
		return false, err
	}
	width, err := m.BitVectorSize(sort)
	if err != nil {
		return false, nil
	}
	return low <= high && high < width, nil
}

func joinGuardErrors(a, b error) error {
	if a != nil {
		return a
	}
	return b
}
