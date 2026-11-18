package qe

// Source: src/qe/qe.cpp, constrain_assignment.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// chooseExprQETreeBranch selects a supported variable and publishes the
// corresponding bound. Unsupported variables are appended to freeVariables.
func chooseExprQETreeBranch(node *exprQETree, variables *exprQEBranchVariables, context *SolverContext, solver *smtcore.SMTSolver, freeVariables *[]ast.ASTRef) (bool, error) {
	if node == nil || node.atoms == nil || solver == nil {
		return false, ast.ErrASTHandle
	}
	// rational num_branches;
	q := node.manager.NumeralService().Manager
	count := unumeral.NewRational()
	variable, found, err := selectExprQEVariable(node, variables, context, node.atoms, freeVariables, &count)
	if err == nil && found {
		err = constrainExprQEBranch(node, variables, solver, variable, &count)
	}
	if err = errors.Join(err, q.Del(&count)); err != nil || !found {
		return false, err
	}
	return true, nil
}
