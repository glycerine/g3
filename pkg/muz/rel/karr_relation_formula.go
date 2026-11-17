package rel

// Source: src/muz/rel/karr_relation.cpp karr_relation::to_formula.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/smtcore"
)

// ToFormula is karr_relation::to_formula: false when empty, otherwise the
// conjunction (bool_rewriter::mk_and) of one linear (in)equality per row of
// get_ineqs(), built by to_formula(row, b, is_eq, conj) over Int variables
// (m.mk_var(i, a.mk_int()) whatever the column's sort). The returned AST
// reference is owned by the caller.
func (r *KarrRelation) ToFormula(manager *ast.ASTManager, sorts []ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	if r == nil || manager == nil || len(sorts) != int(r.dimension) {
		return result, ErrKarrDimension
	}
	if r.empty {
		value, err := manager.MkFalse()
		if err != nil {
			return result, err
		}
		return manager.NewRef(value)
	}
	family, err := manager.RegisterArithmeticPlugin()
	if err != nil {
		return result, err
	}
	matrix, err := r.Inequalities()
	if err != nil {
		return result, err
	}
	// conj: the expr_ref_vector of the row formulas.
	var conj []ast.ASTRef
	owned := false
	defer func() {
		for i := len(conj) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, manager.DelRef(&conj[i]))
		}
		if resultErr != nil && owned {
			resultErr = errors.Join(resultErr, manager.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	handles := make([]ast.ASTHandle, 0, len(matrix.A))
	for rowIndex, row := range matrix.A {
		if len(row) != int(r.dimension) {
			return result, ErrKarrDimension
		}
		formula, err := karrRowFormula(manager, family, row, &matrix.B[rowIndex], matrix.Eq[rowIndex])
		if err != nil {
			return result, err
		}
		conj = append(conj, formula)
		h, err := manager.RefGet(formula)
		if err != nil {
			return result, err
		}
		handles = append(handles, h)
	}
	result, err = manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	owned = true
	if err = rewriter.SimplifyBooleanGateDefault(manager, ast.ASTOpAnd, handles, result); err != nil {
		return result, err
	}
	return result, nil
}

// karrRowFormula is karr_relation::to_formula(row, b, is_eq, conj): the
// pushed formula as an owned reference. Its locals sum, zero and lhs die at
// the return (lhs, zero, then sum).
func karrRowFormula(manager *ast.ASTManager, family ast.FamilyID, row []big.Rat, b *big.Rat, isEq bool) (result ast.ASTRef, resultErr error) {
	var sum []ast.ASTRef
	var zero, lhs ast.ASTRef
	defer func() {
		resultErr = errors.Join(resultErr, manager.DelRef(&lhs), manager.DelRef(&zero))
		for i := len(sum) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, manager.DelRef(&sum[i]))
		}
		if resultErr != nil {
			_ = manager.DelRef(&result)
			result = ast.ASTRef{}
		}
	}()
	hold := func(value ast.ASTHandle, err error) (ast.ASTRef, ast.ASTHandle, error) {
		if err != nil {
			return ast.ASTRef{}, ast.ASTHandle{}, err
		}
		ref, err := manager.NewRef(value)
		return ref, value, err
	}
	zeroValue := ast.ASTHandle{}
	var err error
	if zero, zeroValue, err = hold(manager.ArithmeticIntegerNumeralText("0")); err != nil {
		return result, err
	}
	intSort, err := manager.MkFamilySort(family, 1, nil)
	if err != nil {
		return result, err
	}
	one := big.NewRat(1, 1)
	terms := make([]ast.ASTHandle, 0, len(row)+1)
	for i := range row {
		coefficient := &row[i]
		if coefficient.Sign() == 0 {
			continue
		}
		variable, err := manager.MkVar(uint32(i), intSort)
		if err != nil {
			return result, err
		}
		term := variable
		if coefficient.Cmp(one) != 0 {
			numeral, err := manager.ArithmeticNumeralRat(coefficient, true)
			if err != nil {
				return result, err
			}
			if term, err = manager.MkArithmeticMul(numeral, variable); err != nil {
				return result, err
			}
		}
		ref, h, err := hold(term, nil)
		if err != nil {
			return result, err
		}
		sum = append(sum, ref)
		terms = append(terms, h)
	}
	if b.Sign() != 0 {
		ref, h, err := hold(manager.ArithmeticNumeralRat(b, true))
		if err != nil {
			return result, err
		}
		sum = append(sum, ref)
		terms = append(terms, h)
	}
	// arith_util::mk_add(n, args): args[0] for one argument, the n-ary
	// addition otherwise.
	var lhsValue ast.ASTHandle
	switch len(terms) {
	case 0:
		// A zero row with a zero constant (Karr matrices have none).
		lhsValue = zeroValue
	case 1:
		lhsValue = terms[0]
	default:
		if lhsValue, err = manager.MkFamilyApp(family, 6, nil, terms); err != nil {
			return result, err
		}
	}
	if lhs, lhsValue, err = hold(lhsValue, nil); err != nil {
		return result, err
	}
	var formula ast.ASTHandle
	if isEq {
		formula, err = manager.MkEq(lhsValue, zeroValue)
	} else {
		formula, err = manager.MkArithmeticGE(lhsValue, zeroValue)
	}
	if err != nil {
		return result, err
	}
	return manager.NewRef(formula)
}

func VerifyKarrRelationExpected(manager *ast.ASTManager, sorts []ast.ASTHandle, config smtcore.SMTSolverConfig, expected ast.ASTHandle, actual *KarrRelation) (resultErr error) {
	if manager == nil || expected.IsNull() || actual == nil {
		return ErrRelationVerification
	}
	formula, err := actual.ToFormula(manager, sorts)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&formula)) }()
	actualFormula, err := manager.RefGet(formula)
	if err != nil {
		return err
	}
	return verifyRelationFormulaPair(manager, sorts, config, expected, actualFormula, false)
}
