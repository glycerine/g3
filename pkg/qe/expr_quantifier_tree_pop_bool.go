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
// analysis and variable selection. With exprQEPopNeedsPartition the
// partition pop computed (get_partition and partition_vars run once, AUDIT.md
// qe_02-F8) is returned for process_partition and the caller closes it.
func popExprQEBoolTree(node *exprQETree, variables *exprQEBranchVariables, context *SolverContext, plugin *BoolPlugin, partitioner *ConjunctionPartitioner, solver *smtcore.SMTSolver, evaluator *model.ModelEvaluator, modelCore *model.ModelCore, config Config, rw *rewriter.TheoryRewriter, nnf *QENNFNormalizer, freeVariables *[]ast.ASTRef, withDefinitions bool) (result *exprQETree, split *exprQETreePartition, status exprQEPopStatus, err error) {
	tree := &exprQETreeContext{current: node, variables: variables, config: config, rw: rw, nnf: nnf}
	if err = solveExprQEVars(tree, context); err != nil {
		return nil, nil, exprQEPopNoBranch, err
	}
	current := tree.current
	blocked, err := blockExprQETreeIfSolved(current, solver)
	if err != nil {
		return nil, nil, exprQEPopNoBranch, err
	}
	if blocked {
		return current, nil, exprQEPopCompleted, nil
	}
	part, err := partitionExprQETree(current, variables, partitioner)
	if err != nil {
		return nil, nil, exprQEPopNoBranch, err
	}
	if part.split {
		return current, part, exprQEPopNeedsPartition, nil
	}
	defer func() { err = errors.Join(err, part.Close()) }()
	child, handled, err := closeExprQETreePartition(current, part, solver, evaluator, modelCore, config, withDefinitions)
	if err != nil {
		return nil, nil, exprQEPopNoBranch, err
	}
	if handled {
		return child, nil, exprQEPopCompleted, nil
	}
	// constrain_assignment: when find_min_weight frees every remaining
	// variable no branch is set and final_check goes on; the next pop finds
	// no free variables and blocks the assignment.
	if _, err = chooseExprQETreeBranch(current, variables, context, solver, freeVariables); err != nil {
		return nil, nil, exprQEPopNoBranch, err
	}
	return current, nil, exprQEPopBranchSelected, nil
}
