package qe

// Source: src/qe/qe.cpp, quant_elim_plugin::eval/mk_eq_value/is_eq_value.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// exprQEEvalBranch is quant_elim_plugin::eval: value (a rational of the
// manager's NumeralService().Manager) receives the branch number.
func exprQEEvalBranch(m *ast.ASTManager, eval *model.ModelEvaluator, variable ast.ASTHandle, value *unumeral.Rational) (found bool, err error) {
	if m == nil || eval == nil {
		return false, ast.ErrASTHandle
	}
	ref, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&ref)) }()
	if err = eval.Evaluate(variable, ref); err != nil {
		return false, err
	}
	h, err := m.RefGet(ref)
	if err != nil {
		return false, err
	}
	q := m.NumeralService().Manager
	if m.IsTrue(h) {
		return true, q.SetInt32(value, 1)
	}
	if m.IsFalse(h) {
		return true, q.SetInt32(value, 0)
	}
	_, ok, err := m.BitVectorNumeralRational(h, value)
	if err != nil {
		return false, err
	}
	return ok, nil
}

// exprQEBranchEquality is quant_elim_plugin::mk_eq_value(b, vl).
func exprQEBranchEquality(m *ast.ASTManager, variable ast.ASTHandle, value *unumeral.Rational) (result ast.ASTRef, err error) {
	if m == nil || value == nil {
		return result, ast.ErrASTDomain
	}
	q := m.NumeralService().Manager
	isInt, err := q.IsInt(value)
	if err != nil {
		return result, err
	}
	sign, err := q.Sign(value)
	if err != nil {
		return result, err
	}
	if !isInt || sign < 0 {
		return result, ast.ErrASTDomain
	}
	sort, err := m.GetSort(variable)
	if err != nil {
		return result, err
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return result, err
	}
	if sort == boolean {
		if sign == 0 {
			h, e := m.MkNotSimplified(variable)
			if e != nil {
				return result, e
			}
			return m.NewRef(h)
		}
		if one, e := q.IsOne(value); e != nil {
			return result, e
		} else if one {
			return m.NewRef(variable)
		}
		return result, ast.ErrASTDomain
	}
	width, err := m.BitVectorSize(variable)
	if err != nil {
		return result, err
	}
	n, err := m.BitVectorNumeralFromRational(value, width)
	if err != nil {
		return result, err
	}
	nRef, err := m.NewRef(n)
	if err != nil {
		return result, err
	}
	defer func() {
		err = errors.Join(err, m.DelRef(&nRef))
		if err != nil && result != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	eq, err := m.MkEq(variable, n)
	if err != nil {
		return result, err
	}
	return m.NewRef(eq)
}
