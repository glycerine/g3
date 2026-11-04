package rel

// Source: check_relation_plugin symbolic verification. The oracle depends on
// original column sorts and solver services, independently of relation storage.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type relationFormulaChecker struct {
	manager *ast.ASTManager
	sorts   []ast.ASTHandle
	config  smtcore.SMTSolverConfig
}

// GroundRelationFormula substitutes the source plugin checker's x0, x1, ...
// column constants for relation variables and returns an owned AST ref.
func GroundRelationFormula(manager *ast.ASTManager, sorts []ast.ASTHandle, formula ast.ASTHandle) (ast.ASTRef, error) {
	if manager == nil || formula.IsNull() {
		return ast.ASTRef{}, ErrRelationVerification
	}
	checker := &relationFormulaChecker{manager: manager, sorts: append([]ast.ASTHandle(nil), sorts...)}
	return checker.groundWithNames(formula, true)
}

// VerifyRelationFormulasEquivalent mirrors check_relation_plugin::check_equiv
// for formulas already grounded over the relation's named columns. Source
// treats unknown as inconclusive and rejects only a satisfiable difference.
func VerifyRelationFormulasEquivalent(manager *ast.ASTManager, config smtcore.SMTSolverConfig, left, right ast.ASTHandle) (resultErr error) {
	if manager == nil || left.IsNull() || right.IsNull() {
		return ErrRelationVerification
	}
	equivalence, err := manager.MkEq(left, right)
	if err != nil {
		return err
	}
	equivalenceRef, err := manager.NewRef(equivalence)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&equivalenceRef)) }()
	difference, err := manager.MkNot(equivalence)
	if err != nil {
		return err
	}
	differenceRef, err := manager.NewRef(difference)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&differenceRef)) }()
	solver, err := smtcore.NewSMTSolver(manager, nil, "", config)
	if solver != nil {
		defer func() { resultErr = errors.Join(resultErr, solver.Close()) }()
	}
	if err != nil {
		return err
	}
	if err = solver.Assert(difference); err != nil {
		return err
	}
	status, err := solver.CheckSat(nil)
	if err != nil {
		return err
	}
	if status == lbool.True {
		return ErrRelationVerification
	}
	return nil
}
