package qe

// Source: src/qe/qe.h:106-132; src/qe/qe.cpp:306-342.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

// ConjunctionEnum owns references to the source-order flattening of a
// conjunction. The manager and root are borrowed during construction.
type ConjunctionEnum struct {
	self    *ConjunctionEnum
	manager *ast.ASTManager
	items   []ast.ASTRef
	closed  bool
}

func NewConjunctionEnum(manager *ast.ASTManager, root ast.ASTHandle) (result *ConjunctionEnum, err error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	c := &ConjunctionEnum{manager: manager}
	c.self = c
	defer func() {
		if err != nil {
			err = errors.Join(err, c.Close())
			result = nil
		}
	}()
	flat, err := manager.FlattenAnd([]ast.ASTHandle{root})
	if err != nil {
		return nil, err
	}
	for _, expression := range flat {
		ref, cause := manager.NewRef(expression)
		if cause != nil {
			return nil, cause
		}
		c.items = append(c.items, ref)
	}
	return c, nil
}

func (c *ConjunctionEnum) open() error {
	if c == nil || c.self != c {
		return ast.ErrASTHandle
	}
	if c.closed {
		return arena.ErrClosed
	}
	return nil
}

func (c *ConjunctionEnum) Len() (uint32, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	return uint32(len(c.items)), nil
}

func (c *ConjunctionEnum) At(index uint32) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if index >= uint32(len(c.items)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return c.manager.RefGet(c.items[index])
}

// Basic equality and arithmetic <= use the same numeric operation code in
// different declaration families. Keep equality distinct after recognition.
const qeConjunctionEquality int32 = -1

func (c *ConjunctionEnum) arithmeticRelation(expression ast.ASTHandle) (kind int32, left, right ast.ASTHandle, matched bool, err error) {
	nodeKind, err := c.manager.GetKind(expression)
	if err != nil || nodeKind != ast.ASTApp {
		return 0, left, right, false, err
	}
	family, err := c.manager.GetFamilyID(expression)
	if err != nil {
		return 0, left, right, false, err
	}
	kind, err = c.manager.GetDeclKind(expression)
	if err != nil {
		return 0, left, right, false, err
	}
	if family == c.manager.BasicFamilyID() && kind == ast.ASTOpEq {
		left, err = c.manager.GetArg(expression, 0)
		if err != nil {
			return 0, left, right, false, err
		}
		sort, cause := c.manager.GetSort(left)
		if cause != nil {
			return 0, left, right, false, cause
		}
		arithmetic, cause := c.manager.ArithmeticFamilyID()
		if cause != nil {
			return 0, left, right, false, cause
		}
		sortFamily, cause := c.manager.GetFamilyID(sort)
		if cause != nil || sortFamily != arithmetic {
			return 0, left, right, false, cause
		}
		right, err = c.manager.GetArg(expression, 1)
		return qeConjunctionEquality, left, right, err == nil, err
	}
	arithmetic, cause := c.manager.ArithmeticFamilyID()
	if cause != nil {
		return 0, left, right, false, cause
	}
	if family != arithmetic || (kind != 2 && kind != 3) {
		return kind, left, right, false, nil
	}
	left, err = c.manager.GetArg(expression, 0)
	if err == nil {
		right, err = c.manager.GetArg(expression, 1)
	}
	// arith.is_ge(e, a1, a0) deliberately swaps its output arguments so
	// both relation kinds are represented as a0 <= a1.
	if err == nil && kind == 3 {
		left, right = right, left
	}
	return kind, left, right, err == nil, err
}

// ExtractEqualities is conj_enum::extract_equalities. It returns owning
// references: arithmetic equalities become lhs-rhs; opposite <=/>=
// inequalities become one lhs-rhs equality in the source scan order. The
// temporaries are released as the source's expr_refs are (tmp2, tmp1, then
// the trail front to back).
func (c *ConjunctionEnum) ExtractEqualities() (result []ast.ASTRef, err error) {
	if err = c.open(); err != nil {
		return nil, err
	}
	m := c.manager
	defer func() {
		if err == nil {
			return
		}
		for i := range result {
			err = errors.Join(err, m.DelRef(&result[i]))
		}
		result = nil
	}()
	seen := make(map[ast.ASTHandle]bool)
	var trail []ast.ASTRef
	tmp1, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, err
	}
	tmp2, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, errors.Join(err, m.DelRef(&tmp1))
	}
	defer func() {
		err = errors.Join(err, m.DelRef(&tmp2), m.DelRef(&tmp1))
		for i := range trail {
			err = errors.Join(err, m.DelRef(&trail[i]))
		}
	}()
	push := func(out *[]ast.ASTRef, h ast.ASTHandle) error {
		ref, cause := m.NewRef(h)
		if cause == nil {
			*out = append(*out, ref)
		}
		return cause
	}
	for i := range c.items {
		expression, cause := m.RefGet(c.items[i])
		if cause != nil {
			return nil, cause
		}
		kind, left, right, matched, cause := c.arithmeticRelation(expression)
		if cause != nil {
			return nil, cause
		}
		if !matched {
			continue
		}
		difference, cause := m.MkArithmeticSub(left, right)
		if cause == nil {
			cause = m.SetRef(tmp1, difference)
		}
		if cause != nil {
			return nil, cause
		}
		if kind == qeConjunctionEquality {
			if cause = push(&result, difference); cause != nil {
				return nil, cause
			}
			continue
		}
		reverse, cause := m.MkArithmeticSub(right, left)
		if cause == nil {
			cause = m.SetRef(tmp2, reverse)
		}
		if cause != nil {
			return nil, cause
		}
		if seen[reverse] {
			cause = push(&result, difference)
		} else {
			cause = push(&trail, difference)
			seen[difference] = true
		}
		if cause != nil {
			return nil, cause
		}
	}
	return result, nil
}

func (c *ConjunctionEnum) Close() (err error) {
	if c == nil || c.self != c {
		return ast.ErrASTHandle
	}
	if c.closed {
		return nil
	}
	for len(c.items) > 0 {
		i := len(c.items) - 1
		if cause := c.manager.DelRef(&c.items[i]); cause != nil {
			return cause
		}
		c.items = c.items[:i]
	}
	c.closed = true
	return nil
}
