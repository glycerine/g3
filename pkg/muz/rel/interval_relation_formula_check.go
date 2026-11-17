package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
)

func verifyIntervalRelationExpected(manager *ast.ASTManager, sorts []ast.ASTHandle, config smtcore.SMTSolverConfig, expected ast.ASTHandle, actual *IntervalRelation, named bool) (resultErr error) {
	if manager == nil || expected.IsNull() || actual == nil {
		return ErrRelationVerification
	}
	actualFormula, err := actual.ToFormula(manager, sorts)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&actualFormula)) }()
	actualHandle, err := manager.RefGet(actualFormula)
	if err != nil {
		return err
	}
	return verifyRelationFormulaPair(manager, sorts, config, expected, actualHandle, named)
}

func VerifyIntervalRelationEmpty(manager *ast.ASTManager, sorts []ast.ASTHandle, config smtcore.SMTSolverConfig, relation *IntervalRelation) error {
	if manager == nil || relation == nil {
		return ErrRelationVerification
	}
	if !relation.Empty() {
		return nil
	}
	expected, err := manager.MkFalse()
	if err != nil {
		return err
	}
	ref, err := manager.NewRef(expected)
	if err != nil {
		return err
	}
	defer manager.DelRef(&ref)
	return verifyIntervalRelationExpected(manager, sorts, config, expected, relation, false)
}

// VerifyIntervalRelationFactAdd checks the source check_relation::add_fact
// postcondition against the interval relation's concrete formula.
func VerifyIntervalRelationFactAdd(manager *ast.ASTManager, sorts []ast.ASTHandle, config smtcore.SMTSolverConfig, before ast.ASTHandle, values []ast.ASTHandle, after *IntervalRelation) (resultErr error) {
	if manager == nil || before.IsNull() || len(sorts) != len(values) || after == nil {
		return ErrRelationVerification
	}
	checker := relationFormulaChecker{manager: manager, sorts: append([]ast.ASTHandle(nil), sorts...), config: config}
	fact, err := checker.factFormula(values)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&fact)) }()
	factHandle, err := manager.RefGet(fact)
	if err != nil {
		return err
	}
	expected, err := manager.MkOrArgs([]ast.ASTHandle{before, factHandle})
	if err != nil {
		return err
	}
	expectedRef, err := manager.NewRef(expected)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.DelRef(&expectedRef)) }()
	return verifyIntervalRelationExpected(manager, sorts, config, expected, after, false)
}
