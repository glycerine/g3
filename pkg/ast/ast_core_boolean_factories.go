package ast

// Source: src/ast/ast.h:1984,2008-2018,2031-2060; src/ast/ast_util.cpp:173-180.

func (m *ASTManager) MkFalse() (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.external(m.falseApp), nil
}
func (m *ASTManager) MkTrue() (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	return m.external(m.trueApp), nil
}
func (m *ASTManager) MkNot(a ASTHandle) (ASTHandle, error) {
	args := [1]ASTHandle{a}
	return m.MkFamilyApp(0, ASTOpNot, nil, args[:])
}
func (m *ASTManager) MkOr(a, b ASTHandle) (ASTHandle, error) {
	args := [2]ASTHandle{a, b}
	return m.MkFamilyApp(0, ASTOpOr, nil, args[:])
}
func (m *ASTManager) MkAnd(a, b ASTHandle) (ASTHandle, error) {
	args := [2]ASTHandle{a, b}
	return m.MkFamilyApp(0, ASTOpAnd, nil, args[:])
}

// MkOrArray is ast_util's source overload: singleton returns its raw pointer
// without constructing a new node, and zero uses the pinned false constant.
func (m *ASTManager) MkOrArray(args []ASTHandle) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	switch len(args) {
	case 0:
		return m.external(m.falseApp), nil
	case 1:
		return args[0], nil
	}
	r, e := m.mkFamilyApp(0, ASTOpOr, nil, args)
	return m.external(r), e
}

// IsNot includes the source argument-output overload used by ast_ext2.
func (m *ASTManager) IsNot(a ASTHandle) (bool, ASTHandle, error) {
	if m.fastHandle(a) { // readNode's unchecked case
		return m.isNot(astNodeRef{a.slot, a.generation})
	}
	if e := m.lock(); e != nil {
		return false, ASTHandle{}, e
	}
	defer m.mu.Unlock()
	r, e := m.internal(a)
	if e != nil {
		return false, ASTHandle{}, e
	}
	return m.isNot(r)
}
func (m *ASTManager) isNot(r astNodeRef) (bool, ASTHandle, error) {
	if m.nodeAt(r.slot).kind != ASTApp {
		return false, ASTHandle{}, nil
	}
	app := m.appAt(r.slot)
	family, kind, e := m.appIdentity(app)
	if e != nil {
		return false, ASTHandle{}, e
	}
	if family != FamilyID(0) || kind != ASTOpNot {
		return false, ASTHandle{}, nil
	}
	child, e := m.appArg(app, 0)
	return true, m.external(child), e
}

// DistinctFlatBooleanOperands recognizes an AND or OR that needs neither
// flattening nor constant, duplicate, or complement simplification. The caller
// supplies ASTOpAnd or ASTOpOr. Returning false leaves the general reducer
// responsible for checking and simplifying the operands.
func (m *ASTManager) DistinctFlatBooleanOperands(args AppArguments, op int32) bool {
	var atoms [8]astNodeRef
	n := args.count
	if astValidateHandles || n < 2 || n > uint32(len(atoms)) || args.appOwner != nil || args.raw ||
		uint64(args.argumentOffset)+uint64(n) > uint64(len(args.slice)) || m == nil || !m.lockFree() {
		return false
	}
	for i, h := range args.slice[args.argumentOffset : args.argumentOffset+n] {
		r, e := m.internal(h)
		if e != nil || r == m.trueApp || r == m.falseApp {
			return false
		}
		if m.nodeAt(r.slot).kind == ASTApp {
			a := m.appAt(r.slot)
			family, kind, e := m.appIdentity(a)
			if e != nil {
				return false
			}
			if family == 0 {
				if kind == op {
					return false
				}
				if kind == ASTOpNot {
					r, e = m.appArg(a, 0)
					if e != nil {
						return false
					}
				}
			}
		}
		// Either polarity of a repeated atom requires the general reducer.
		for _, seen := range atoms[:i] {
			if seen == r {
				return false
			}
		}
		atoms[i] = r
	}
	return true
}
