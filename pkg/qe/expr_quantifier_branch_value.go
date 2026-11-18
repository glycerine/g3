package qe

// Source: src/qe/qe.cpp, quant_elim_plugin::eval/mk_eq_value/is_eq_value.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

func exprQEEvalBranch(m *ast.ASTManager, eval *model.ModelEvaluator, variable ast.ASTHandle) (value big.Rat, found bool, err error) {
	if m == nil || eval == nil {
		return value, false, ast.ErrASTHandle
	}
	ref, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return value, false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&ref)) }()
	if err = eval.Evaluate(variable, ref); err != nil {
		return value, false, err
	}
	h, err := m.RefGet(ref)
	if err != nil {
		return value, false, err
	}
	if m.IsTrue(h) {
		value.SetInt64(1)
		return value, true, nil
	}
	if m.IsFalse(h) {
		value.SetInt64(0)
		return value, true, nil
	}
	n, _, ok, err := m.BitVectorNumeralBigInt(h)
	if err != nil {
		return value, false, err
	}
	if ok {
		value.SetInt(n)
		return value, true, nil
	}
	return value, false, nil
}

func exprQEBranchEquality(m *ast.ASTManager, variable ast.ASTHandle, value *big.Rat) (result ast.ASTRef, err error) {
	if m == nil || value == nil || !value.IsInt() || value.Sign() < 0 {
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
		if value.Sign() == 0 {
			h, e := m.MkNotSimplified(variable)
			if e != nil {
				return result, e
			}
			return m.NewRef(h)
		}
		if value.Cmp(big.NewRat(1, 1)) == 0 {
			return m.NewRef(variable)
		}
		return result, ast.ErrASTDomain
	}
	width, err := m.BitVectorSize(variable)
	if err != nil {
		return result, err
	}
	n, err := m.BitVectorNumeralRat(value, width)
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
