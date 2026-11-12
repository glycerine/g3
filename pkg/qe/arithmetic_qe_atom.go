package qe

// Source: src/qe/qe_arith_plugin.cpp arith_qe_util::mk_atom.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

func (u *ArithmeticQETermUtil) relation(expression ast.ASTHandle) (kind int32, left, right ast.ASTHandle, arithmetic bool, err error) {
	family, err := u.manager.GetFamilyID(expression)
	if err != nil {
		return 0, left, right, false, err
	}
	kind, err = u.manager.GetDeclKind(expression)
	if err != nil {
		return 0, left, right, false, err
	}
	if family != u.family || kind < 2 || kind > 5 {
		return kind, left, right, false, nil
	}
	left, err = u.manager.GetArg(expression, 0)
	if err == nil {
		right, err = u.manager.GetArg(expression, 1)
	}
	return kind, left, right, err == nil, err
}

func (u *ArithmeticQETermUtil) strictZero(expression ast.ASTHandle) (ast.ASTRef, error) {
	return u.mkLt(expression)
}

// NormalizeAtom retains equalities and converts arithmetic atoms to the
// source zero-comparison normal forms for the requested polarity.
func (u *ArithmeticQETermUtil) NormalizeAtom(expression ast.ASTHandle, positive bool) (result ast.ASTRef, handled bool, resultErr error) {
	if resultErr = u.open(); resultErr != nil {
		return result, false, resultErr
	}
	if _, _, divides, err := u.MatchDivides(expression); err != nil {
		return result, false, err
	} else if divides {
		if positive {
			result, err = u.manager.NewRef(expression)
			return result, err == nil, err
		}
		negative, err := u.manager.MkNot(expression)
		if err != nil {
			return result, false, err
		}
		result, err = u.manager.NewRef(negative)
		return result, err == nil, err
	}
	args, equality, err := u.operation(expression, u.manager.BasicFamilyID(), ast.ASTOpEq)
	if err != nil {
		return result, false, err
	}
	if equality && len(args) == 2 {
		sort, cause := u.manager.GetSort(args[0])
		if cause != nil {
			return result, false, cause
		}
		family, cause := u.manager.GetFamilyID(sort)
		if cause != nil || family != u.family {
			return result, false, cause
		}
		if positive {
			return u.atomPair(args[0], args[1], false, true)
		}
		integer, cause := u.isInteger(args[0])
		if cause != nil {
			return result, false, cause
		}
		if integer {
			return u.atomIntDisequality(args[0], args[1])
		}
		return u.atomPair(args[0], args[1], true, false)
	}
	kind, left, right, relation, err := u.relation(expression)
	if err != nil || !relation {
		return result, false, err
	}
	if positive && (kind == 2 || kind == 3) {
		result, err = u.manager.NewRef(expression)
		return result, err == nil, err
	}
	if kind == 3 || kind == 5 {
		left, right = right, left
	}
	if !positive {
		left, right = right, left
	}
	difference, err := u.manager.MkArithmeticSub(left, right)
	if err != nil {
		return result, false, err
	}
	// tmp1 = mk_sub(...): the expr_ref lives until mk_atom returns.
	tmp1, err := u.manager.NewRef(difference)
	if err != nil {
		return result, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, u.manager.DelRef(&tmp1)) }()
	strict := positive == (kind == 4 || kind == 5)
	if strict {
		result, err = u.strictZero(difference)
	} else {
		result, err = u.comparisonZero(difference, false)
	}
	return result, err == nil, err
}

// atomPair is mk_atom's two-sided equality case: t1 = a0 - a1 and
// t2 = a1 - a0, each simplified, compared with zero and combined by the
// Boolean rewriter. Locals end in reverse declaration order.
func (u *ArithmeticQETermUtil) atomPair(a0, a1 ast.ASTHandle, strict, conjunction bool) (result ast.ASTRef, handled bool, resultErr error) {
	m := u.manager
	var t1, t2, tmp1, tmp2 ast.ASTRef
	defer func() {
		for _, r := range []*ast.ASTRef{&tmp2, &tmp1, &t2, &t1} {
			if *r != (ast.ASTRef{}) {
				resultErr = errors.Join(resultErr, m.DelRef(r))
			}
		}
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, m.DelRef(&result))
		}
	}()
	var err error
	if t1, err = u.subSimplified(a0, a1); err != nil {
		return result, false, err
	}
	if t2, err = u.subSimplified(a1, a0); err != nil {
		return result, false, err
	}
	if tmp1, err = u.compareRef(t1, strict); err != nil {
		return result, false, err
	}
	if tmp2, err = u.compareRef(t2, strict); err != nil {
		return result, false, err
	}
	operation := int32(ast.ASTOpOr)
	if conjunction {
		operation = ast.ASTOpAnd
	}
	result, err = u.boolGate(operation, tmp1, tmp2)
	return result, err == nil, err
}

// atomIntDisequality is mk_atom's !p integer equality case:
// 1 + a0 - a1 <= 0 or 1 - (a0 - a1) <= 0.
func (u *ArithmeticQETermUtil) atomIntDisequality(a0, a1 ast.ASTHandle) (result ast.ASTRef, handled bool, resultErr error) {
	m := u.manager
	var t1, t2, tmp1, tmp2 ast.ASTRef
	defer func() {
		for _, r := range []*ast.ASTRef{&tmp2, &tmp1, &t2, &t1} {
			if *r != (ast.ASTRef{}) {
				resultErr = errors.Join(resultErr, m.DelRef(r))
			}
		}
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, m.DelRef(&result))
		}
	}()
	one, err := m.ArithmeticIntegerNumeralText("1")
	if err != nil {
		return result, false, err
	}
	difference, err := m.MkArithmeticSub(a0, a1)
	if err != nil {
		return result, false, err
	}
	if tmp1, err = m.NewRef(difference); err != nil {
		return result, false, err
	}
	sum, err := m.MkArithmeticAdd(one, difference)
	if err != nil {
		return result, false, err
	}
	if t1, err = u.simplifiedRef(sum); err != nil {
		return result, false, err
	}
	reverse, err := m.MkArithmeticSub(one, difference)
	if err != nil {
		return result, false, err
	}
	if t2, err = u.simplifiedRef(reverse); err != nil {
		return result, false, err
	}
	le, err := u.compareRef(t1, false)
	if err != nil {
		return result, false, err
	}
	err = m.AssignRef(tmp1, le)
	err = errors.Join(err, m.DelRef(&le))
	if err != nil {
		return result, false, err
	}
	if tmp2, err = u.compareRef(t2, false); err != nil {
		return result, false, err
	}
	result, err = u.boolGate(ast.ASTOpOr, tmp1, tmp2)
	return result, err == nil, err
}

// simplifiedRef is expr_ref t(e); simplify(t).
func (u *ArithmeticQETermUtil) simplifiedRef(expression ast.ASTHandle) (ast.ASTRef, error) {
	ref, err := u.manager.NewRef(expression)
	if err != nil {
		return ref, err
	}
	if err = u.theory.Rewrite(expression, ref); err != nil {
		return ast.ASTRef{}, errors.Join(err, u.manager.DelRef(&ref))
	}
	return ref, nil
}

func (u *ArithmeticQETermUtil) subSimplified(a, b ast.ASTHandle) (ast.ASTRef, error) {
	difference, err := u.manager.MkArithmeticSub(a, b)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return u.simplifiedRef(difference)
}

func (u *ArithmeticQETermUtil) compareRef(ref ast.ASTRef, strict bool) (ast.ASTRef, error) {
	h, err := u.manager.RefGet(ref)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return u.comparisonZero(h, strict)
}

// boolGate is m_bool_rewriter.mk_and/mk_or of two expr_refs.
func (u *ArithmeticQETermUtil) boolGate(operation int32, left, right ast.ASTRef) (ast.ASTRef, error) {
	m := u.manager
	l, err := m.RefGet(left)
	if err != nil {
		return ast.ASTRef{}, err
	}
	r, err := m.RefGet(right)
	if err != nil {
		return ast.ASTRef{}, err
	}
	out, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return out, err
	}
	if err = astrewriter.SimplifyBooleanGateDefault(m, operation, []ast.ASTHandle{l, r}, out); err != nil {
		return ast.ASTRef{}, errors.Join(err, m.DelRef(&out))
	}
	return out, nil
}
