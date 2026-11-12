package qe

// Source: src/qe/qe.cpp, quant_elim_plugin::pop.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
)

type exprQEPopStatus uint8

const (
	exprQEPopCompleted exprQEPopStatus = iota
	exprQEPopNeedsPartition
	exprQEPopBranchSelected
	exprQEPopNoBranch
)

// popExprQEBoolTree is quant_elim_plugin::pop: solve_vars, then completed
// paths are blocked and remaining nodes are routed through partition
// analysis and variable selection.
func popExprQEBoolTree(node *exprQETree, variables *exprQEBranchVariables, context *SolverContext, plugin *BoolPlugin, partitioner *ConjunctionPartitioner, solver *smtcore.SMTSolver, evaluator *model.ModelEvaluator, modelCore *model.ModelCore, config Config, rw *rewriter.TheoryRewriter, nnf *QENNFNormalizer, freeVariables *[]ast.ASTRef, withDefinitions bool) (result *exprQETree, status exprQEPopStatus, err error) {
	tree := &exprQETreeContext{current: node, variables: variables, config: config, rw: rw, nnf: nnf}
	if err = solveExprQEVars(tree, context); err != nil {
		return nil, exprQEPopNoBranch, err
	}
	current := tree.current
	blocked, err := blockExprQETreeIfSolved(current, solver)
	if err != nil {
		return nil, exprQEPopNoBranch, err
	}
	if blocked {
		return current, exprQEPopCompleted, nil
	}
	part, err := partitionExprQETree(current, variables, partitioner)
	if err != nil {
		return nil, exprQEPopNoBranch, err
	}
	defer func() { err = errors.Join(err, part.Close()) }()
	if part.split {
		return current, exprQEPopNeedsPartition, nil
	}
	child, handled, err := closeExprQETreePartition(current, part, solver, evaluator, modelCore, config, withDefinitions)
	if err != nil {
		return nil, exprQEPopNoBranch, err
	}
	if handled {
		return child, exprQEPopCompleted, nil
	}
	// constrain_assignment: when find_min_weight frees every remaining
	// variable no branch is set and final_check goes on; the next pop finds
	// no free variables and blocks the assignment.
	if _, err = chooseExprQETreeBranch(current, variables, context, solver, freeVariables); err != nil {
		return nil, exprQEPopNoBranch, err
	}
	return current, exprQEPopBranchSelected, nil
}
