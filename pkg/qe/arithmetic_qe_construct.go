package qe

// Source: src/qe/qe_arith_plugin.cpp arith_qe_util constructors.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (u *ArithmeticQETermUtil) scale(value *big.Rat, term ast.ASTHandle) (ast.ASTRef, error) {
	if value == nil {
		return ast.ASTRef{}, ast.ErrASTHandle
	}
	if value.Cmp(big.NewRat(1, 1)) == 0 {
		return u.manager.NewRef(term)
	}
	numeral, err := u.numeralFor(value, term)
	if err != nil {
		return ast.ASTRef{}, err
	}
	product, err := u.manager.MkArithmeticMul(numeral, term)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return u.manager.NewRef(product)
}

// comparisonZero is arith_qe_util::mk_lt(e) when strict and mk_le(e)
// otherwise. The caller owns the returned reference.
func (u *ArithmeticQETermUtil) comparisonZero(expression ast.ASTHandle, strict bool) (ast.ASTRef, error) {
	if strict {
		return u.mkLt(expression)
	}
	return u.mkLe(expression)
}

// mkLe is arith_qe_util::mk_le: simplify(e), then
// m_arith_rewriter.mk_le(e, 0).
func (u *ArithmeticQETermUtil) mkLe(expression ast.ASTHandle) (result ast.ASTRef, err error) {
	if u.theory == nil || u.arith == nil {
		return result, ast.ErrASTHandle
	}
	m := u.manager
	// mk_zero(e) reads e's sort; e may be released by the rewrite.
	zero, err := u.zero(expression)
	if err != nil {
		return result, err
	}
	tmp, err := m.NewRef(expression)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&tmp)) }()
	if err = u.theory.Rewrite(expression, tmp); err != nil {
		return result, err
	}
	simplified, err := m.RefGet(tmp)
	if err != nil {
		return result, err
	}
	return u.arith.MkLe2(simplified, zero)
}

// mkLt is arith_qe_util::mk_lt.
func (u *ArithmeticQETermUtil) mkLt(expression ast.ASTHandle) (result ast.ASTRef, err error) {
	if u.theory == nil {
		return result, ast.ErrASTHandle
	}
	m := u.manager
	var value ast.ASTHandle
	if r, numeral, e := u.numeral(expression); e != nil {
		return result, e
	} else if numeral {
		value, err = m.MkBoolVal(r.Sign() < 0)
	} else if integer, e := u.isInteger(expression); e != nil {
		return result, e
	} else if integer {
		var minusOne ast.ASTHandle
		if minusOne, err = m.ArithmeticIntegerNumeralText("-1"); err == nil {
			value, err = m.MkArithmeticLE(expression, minusOne)
		}
	} else {
		var zero, le ast.ASTHandle
		if zero, err = u.zero(expression); err == nil {
			le, err = m.MkArithmeticLE(zero, expression)
		}
		if err == nil {
			value, err = m.MkNot(le)
		}
	}
	if err != nil {
		return result, err
	}
	if result, err = m.NewRef(value); err != nil {
		return result, err
	}
	if err = u.theory.Rewrite(value, result); err != nil {
		return result, errors.Join(err, m.DelRef(&result))
	}
	return result, nil
}

// MakeBound constructs b*t <= a*s. Strict integer bounds use e+1 <= 0.
func (u *ArithmeticQETermUtil) MakeBound(a *big.Rat, t ast.ASTHandle, b *big.Rat, s ast.ASTHandle, strict bool) (result ast.ASTRef, resultErr error) {
	if resultErr = u.open(); resultErr != nil {
		return result, resultErr
	}
	if a == nil || b == nil || a.Sign() != b.Sign() {
		return result, ast.ErrASTDomain
	}
	absA, absB := new(big.Rat).Abs(a), new(big.Rat).Abs(b)
	ss, err := u.scale(absA, s)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, u.manager.DelRef(&ss)) }()
	tt, err := u.scale(absB, t)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, u.manager.DelRef(&tt)) }()
	sh, err := u.manager.RefGet(ss)
	if err != nil {
		return result, err
	}
	th, err := u.manager.RefGet(tt)
	if err != nil {
		return result, err
	}
	left, right := sh, th
	if a.Sign() < 0 {
		left, right = th, sh
	}
	difference, err := u.manager.MkArithmeticSub(left, right)
	if err != nil {
		return result, err
	}
	differenceRef, err := u.manager.NewRef(difference)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, u.manager.DelRef(&differenceRef)) }()
	integer, err := u.isInteger(difference)
	if err != nil {
		return result, err
	}
	if strict && integer {
		one, cause := u.manager.ArithmeticIntegerNumeralText("1")
		if cause != nil {
			return result, cause
		}
		shifted, cause := u.manager.MkArithmeticAdd(difference, one)
		if cause != nil {
			return result, cause
		}
		shiftedRef, cause := u.manager.NewRef(shifted)
		if cause != nil {
			return result, cause
		}
		comparison, cause := u.comparisonZeroOwned(shifted, false)
		return comparison, errors.Join(cause, u.manager.DelRef(&shiftedRef))
	}
	return u.comparisonZeroOwned(difference, strict)
}

func (u *ArithmeticQETermUtil) comparisonZeroOwned(expression ast.ASTHandle, strict bool) (ast.ASTRef, error) {
	return u.comparisonZero(expression, strict)
}

// MakeAbs retains the source numeral shortcut and ite fallback.
func (u *ArithmeticQETermUtil) MakeAbs(expression ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	if resultErr = u.open(); resultErr != nil {
		return result, resultErr
	}
	if value, numeral, err := u.numeral(expression); err != nil {
		return result, err
	} else if numeral {
		if value.Sign() >= 0 {
			return u.manager.NewRef(expression)
		}
		negative, err := u.manager.MkArithmeticNeg(expression)
		if err != nil {
			return result, err
		}
		return u.manager.NewRef(negative)
	}
	zero, err := u.zero(expression)
	if err != nil {
		return result, err
	}
	nonnegative, err := u.manager.MkArithmeticLE(zero, expression)
	if err != nil {
		return result, err
	}
	nonnegativeRef, err := u.manager.NewRef(nonnegative)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, u.manager.DelRef(&nonnegativeRef)) }()
	negative, err := u.manager.MkArithmeticNeg(expression)
	if err != nil {
		return result, err
	}
	abs, err := u.manager.MkIte(nonnegative, expression, negative)
	if err != nil {
		return result, err
	}
	return u.manager.NewRef(abs)
}

func (u *ArithmeticQETermUtil) minMax(arguments []ast.ASTHandle, maximum bool) (result ast.ASTRef, resultErr error) {
	if resultErr = u.open(); resultErr != nil {
		return result, resultErr
	}
	if len(arguments) == 0 {
		return result, ast.ErrASTDomain
	}
	result, resultErr = u.manager.NewRef(arguments[len(arguments)-1])
	if resultErr != nil {
		return result, resultErr
	}
	for i := len(arguments) - 2; i >= 0; i-- {
		current, err := u.manager.RefGet(result)
		if err != nil {
			return result, errors.Join(err, u.manager.DelRef(&result))
		}
		left, right := arguments[i], current
		if !maximum {
			left, right = current, arguments[i]
		}
		comparison, err := u.manager.MkArithmeticLE(left, right)
		if err != nil {
			return result, errors.Join(err, u.manager.DelRef(&result))
		}
		choice, err := u.manager.MkIte(comparison, current, arguments[i])
		if err != nil {
			return result, errors.Join(err, u.manager.DelRef(&result))
		}
		if err = u.manager.SetRef(result, choice); err != nil {
			return result, errors.Join(err, u.manager.DelRef(&result))
		}
	}
	return result, nil
}

func (u *ArithmeticQETermUtil) MakeMax(arguments []ast.ASTHandle) (ast.ASTRef, error) {
	return u.minMax(arguments, true)
}

func (u *ArithmeticQETermUtil) MakeMin(arguments []ast.ASTHandle) (ast.ASTRef, error) {
	return u.minMax(arguments, false)
}

// MakeDivides is arith_qe_util::mk_divides: 0 = (mod simplify(e) n).
func (u *ArithmeticQETermUtil) MakeDivides(divisor *big.Rat, expression ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	if resultErr = u.open(); resultErr != nil {
		return result, resultErr
	}
	if divisor == nil || !divisor.IsInt() {
		return result, ast.ErrASTDomain
	}
	if u.theory == nil || u.arith == nil || u.boolRw == nil {
		return result, ast.ErrASTHandle
	}
	m := u.manager
	tmp1, err := m.NewRef(expression)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&tmp1)) }()
	if err = u.theory.Rewrite(expression, tmp1); err != nil {
		return result, err
	}
	simplified, err := m.RefGet(tmp1)
	if err != nil {
		return result, err
	}
	n, err := m.ArithmeticNumeralRat(divisor, true)
	if err != nil {
		return result, err
	}
	tmp2, err := u.arith.MkMod2(simplified, n)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&tmp2)) }()
	modular, err := m.RefGet(tmp2)
	if err != nil {
		return result, err
	}
	zero, err := m.ArithmeticNumeralRat(new(big.Rat), true)
	if err != nil {
		return result, err
	}
	if result, err = m.NewRef(ast.ASTHandle{}); err != nil {
		return result, err
	}
	if err = u.boolRw.MkEq(zero, modular, result); err != nil {
		return result, errors.Join(err, m.DelRef(&result))
	}
	return result, nil
}

// MakeFlatAnd is arith_qe_util::mk_flat_and: the conjuncts of both
// arguments through m_bool_rewriter.mk_and.
func (u *ArithmeticQETermUtil) MakeFlatAnd(left, right ast.ASTHandle) (result ast.ASTRef, err error) {
	if err = u.open(); err != nil {
		return result, err
	}
	if u.boolRw == nil {
		return result, ast.ErrASTHandle
	}
	parts := make([]ast.ASTHandle, 0, 4)
	for _, expression := range []ast.ASTHandle{left, right} {
		args, conjunction, err := u.operation(expression, u.manager.BasicFamilyID(), ast.ASTOpAnd)
		if err != nil {
			return result, err
		}
		if conjunction {
			parts = append(parts, args...)
		} else {
			parts = append(parts, expression)
		}
	}
	if result, err = u.manager.NewRef(ast.ASTHandle{}); err != nil {
		return result, err
	}
	if err = u.boolRw.MkAnd(parts, result); err != nil {
		return result, errors.Join(err, u.manager.DelRef(&result))
	}
	return result, nil
}
