package qe

// Source: src/qe/qe_arith_plugin.cpp arith_qe_util constructors.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// scale is arith_qe_util::mk_mul(k, e): e itself for a unit k.
func (u *ArithmeticQETermUtil) scale(value *unumeral.Rational, term ast.ASTHandle) (ast.ASTRef, error) {
	if value == nil {
		return ast.ASTRef{}, ast.ErrASTHandle
	}
	if one, err := u.q().IsOne(value); err != nil {
		return ast.ASTRef{}, err
	} else if one {
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
	// rational r;
	r := unumeral.NewRational()
	defer func() { err = errors.Join(err, u.q().Del(&r)) }()
	if numeral, e := u.numeral(expression, &r); e != nil {
		return result, e
	} else if numeral {
		var negative bool
		if negative, err = u.q().IsNeg(&r); err == nil {
			value, err = m.MkBoolVal(negative)
		}
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
func (u *ArithmeticQETermUtil) MakeBound(a *unumeral.Rational, t ast.ASTHandle, b *unumeral.Rational, s ast.ASTHandle, strict bool) (result ast.ASTRef, resultErr error) {
	if resultErr = u.open(); resultErr != nil {
		return result, resultErr
	}
	if a == nil || b == nil {
		return result, ast.ErrASTDomain
	}
	q := u.q()
	aNegative, err := q.IsNeg(a)
	if err != nil {
		return result, err
	}
	bNegative, err := q.IsNeg(b)
	if err != nil {
		return result, err
	}
	if aNegative != bNegative {
		return result, ast.ErrASTDomain
	}
	// rational abs_a(a); rational abs_b(b);
	absA, absB := unumeral.NewRational(), unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, q.Del(&absA), q.Del(&absB)) }()
	if err = errors.Join(q.Copy(&absA, a), q.Copy(&absB, b), q.Abs(&absA), q.Abs(&absB)); err != nil {
		return result, err
	}
	ss, err := u.scale(&absA, s)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, u.manager.DelRef(&ss)) }()
	tt, err := u.scale(&absB, t)
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
	if aNegative {
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
	differenceHeld := true
	defer func() {
		if differenceHeld {
			resultErr = errors.Join(resultErr, u.manager.DelRef(&differenceRef))
		}
	}()
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
		// e = mk_add(e, m_one_i) drops the expr_ref's hold on the
		// difference, so simplify sees it shared only by the sum (the
		// rewriter caches only shared subterms).
		differenceHeld = false
		if cause = u.manager.DelRef(&differenceRef); cause != nil {
			return result, errors.Join(cause, u.manager.DelRef(&shiftedRef))
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
	// rational val;
	value := unumeral.NewRational()
	defer func() { resultErr = errors.Join(resultErr, u.q().Del(&value)) }()
	if numeral, err := u.numeral(expression, &value); err != nil {
		return result, err
	} else if numeral {
		if nonnegative, err := u.q().IsNonNeg(&value); err != nil {
			return result, err
		} else if nonnegative {
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
func (u *ArithmeticQETermUtil) MakeDivides(divisor *unumeral.Rational, expression ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	if resultErr = u.open(); resultErr != nil {
		return result, resultErr
	}
	if result, resultErr = u.manager.NewRef(ast.ASTHandle{}); resultErr != nil {
		return result, resultErr
	}
	if resultErr = u.makeDividesInto(divisor, expression, result); resultErr != nil {
		return result, errors.Join(resultErr, u.manager.DelRef(&result))
	}
	return result, nil
}

// makeDividesInto is mk_divides(n, e, result) assigning the caller's
// expr_ref result while mk_divides' tmp1 and tmp2 are still alive.
func (u *ArithmeticQETermUtil) makeDividesInto(divisor *unumeral.Rational, expression ast.ASTHandle, result ast.ASTRef) (resultErr error) {
	if divisor == nil {
		return ast.ErrASTDomain
	}
	if isInt, err := u.q().IsInt(divisor); err != nil || !isInt {
		return errors.Join(err, ast.ErrASTDomain)
	}
	if u.theory == nil || u.arith == nil || u.boolRw == nil {
		return ast.ErrASTHandle
	}
	m := u.manager
	tmp1, err := m.NewRef(expression)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&tmp1)) }()
	if err = u.theory.Rewrite(expression, tmp1); err != nil {
		return err
	}
	simplified, err := m.RefGet(tmp1)
	if err != nil {
		return err
	}
	n, err := m.ArithmeticNumeralFromRational(divisor, true)
	if err != nil {
		return err
	}
	tmp2, err := u.arith.MkMod2(simplified, n)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, m.DelRef(&tmp2)) }()
	modular, err := m.RefGet(tmp2)
	if err != nil {
		return err
	}
	zero, err := m.ArithmeticIntegerNumeralText("0")
	if err != nil {
		return err
	}
	return u.boolRw.MkEq(zero, modular, result)
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
