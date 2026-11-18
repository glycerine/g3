package qe

// Source: src/qe/qe.cpp, update_current with apply=true for scalar plugins.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// advanceExprQEScalarTree connects model-guided traversal with the two stages
// of scalar branch propagation. The normalizer must match the child variables.
func advanceExprQEScalarTree(root *exprQETree, variables *exprQEBranchVariables, context *SolverContext, evaluator *model.ModelEvaluator, config Config, rw *rewriter.TheoryRewriter, nnf *QENNFNormalizer, solver *smtcore.SMTSolver, withDefinitions bool) (_ *exprQETree, _ exprQETreeStatus, err error) {
	if root == nil || root.manager == nil || context == nil || context.manager != root.manager {
		return nil, exprQEChooseVariable, ast.ErrASTHandle
	}
	q := root.manager.NumeralService().Manager
	branch := unumeral.NewRational()
	defer func() { err = errors.Join(err, q.Del(&branch)) }()
	current, status, err := seekExprQETree(root, variables, evaluator, &branch, config)
	if err != nil {
		return nil, status, err
	}
	parent := current
	if status == exprQEChooseVariable {
		if current.formula != (ast.ASTRef{}) {
			return current, status, nil
		}
		parent = current.parent
	}
	if parent == nil || parent.selected == (ast.ASTRef{}) {
		return nil, status, ast.ErrASTDomain
	}
	variable, err := parent.manager.RefGet(parent.selected)
	if err != nil {
		return nil, status, err
	}
	plugin, err := context.PluginFor(variable)
	if err != nil {
		return nil, status, err
	}
	return stepExprQEScalarBranch(parent, variables, plugin, &branch, config, rw, nnf, solver, withDefinitions)
}
