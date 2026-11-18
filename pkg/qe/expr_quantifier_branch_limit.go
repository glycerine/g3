package qe

// Source: src/qe/qe.cpp, quant_elim_plugin::constrain_assignment.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// count is a rational of the manager's NumeralService().Manager.
func constrainExprQEBranch(node *exprQETree, variables *exprQEBranchVariables, solver *smtcore.SMTSolver, variable ast.ASTHandle, count *unumeral.Rational) (err error) {
	if node == nil || node.manager == nil || variables == nil || variables.manager != node.manager || solver == nil || count == nil {
		return ast.ErrASTDomain
	}
	m := node.manager
	q := m.NumeralService().Manager
	if isInt, e := q.IsInt(count); e != nil || !isInt {
		return errors.Join(e, ast.ErrASTDomain)
	}
	if positive, e := q.IsPos(count); e != nil || !positive {
		return errors.Join(e, ast.ErrASTDomain)
	}
	branch, err := variables.branch(variable)
	if err != nil {
		return err
	}
	isBV, err := m.IsBitVector(variable)
	if err != nil {
		return err
	}
	if err = node.selectVariable(variable, count); err != nil {
		return err
	}
	if isBV {
		return nil
	}
	sort, err := m.GetSort(branch)
	if err != nil {
		return err
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return err
	}
	if sort == boolean {
		// SASSERT(num_branches <= rational(2))
		if small, e := q.IsInt32(count); e != nil {
			return e
		} else if !small {
			return ast.ErrASTDomain
		}
		if v, e := q.Int32(count); e != nil || v > 2 {
			return errors.Join(e, ast.ErrASTDomain)
		}
		return nil
	}
	width, err := m.BitVectorSize(branch)
	if err != nil {
		return err
	}
	// expr_ref max_val(m_bv.mk_numeral(num_branches - rational::one(), sz), m)
	maxValue := unumeral.NewRational()
	err = errors.Join(q.Copy(&maxValue, count), q.Dec(&maxValue))
	var n ast.ASTHandle
	if err == nil {
		n, err = m.BitVectorNumeralFromRational(&maxValue, width)
	}
	if err = errors.Join(err, q.Del(&maxValue)); err != nil {
		return err
	}
	nRef, err := m.NewRef(n)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&nRef)) }()
	bound, err := m.MkBitVectorULE(branch, n)
	if err != nil {
		return err
	}
	// expr_ref ub(m_bv.mk_ule(b, max_val), m) keeps the literal alive across
	// add_constraint, whose m_literals.reset() may drop the last reference.
	ub, err := m.NewRef(bound)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&ub)) }()
	return assertExprQETreeConstraint(node, solver, true, bound)
}
