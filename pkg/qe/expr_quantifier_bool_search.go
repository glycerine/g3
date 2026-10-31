package qe

// Source: src/qe/qe.cpp, quant_elim_plugin::check/final_check Boolean subset.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// searchExprQEBoolTree explores model-selected Boolean assignments. Recursive
// independent partitions and unsupported plugin branches report incomplete.
func searchExprQEBoolTree(root *exprQETree, variables *exprQEBranchVariables, context *SolverContext, plugin *BoolPlugin, partitioner *ConjunctionPartitioner, solver *smtcore.SMTSolver, config Config, rw *rewriter.TheoryRewriter, nnf *QENNFNormalizer) (result ast.ASTRef, complete bool, err error) {
	if root == nil || root.manager == nil || root.formula == (ast.ASTRef{}) || solver == nil || config.Cooperation == nil {
		return result, false, ast.ErrASTHandle
	}
	m := root.manager
	defer func() {
		if err != nil && result != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&result))
		}
	}()
	var free []ast.ASTRef
	defer func() {
		for i := range free {
			err = errors.Join(err, m.DelRef(&free[i]))
		}
	}()
	for {
		canceled, e := m.Canceled()
		if e != nil {
			return result, false, e
		}
		if canceled {
			return result, false, tactic.NewTacticException(m.ResourceLimit().CancelMessage())
		}
		if e = config.Cooperation.Checkpoint("qe"); e != nil {
			return result, false, e
		}
		status, e := solver.CheckSat(nil)
		if e != nil {
			return result, false, e
		}
		if status == lbool.Undef {
			return result, false, nil
		}
		if status == lbool.False {
			break
		}
		md, e := solver.AcquireModel()
		if e != nil || md == nil {
			return result, false, errors.Join(e, ast.ErrASTDomain)
		}
		eval, e := model.NewModelEvaluator(md, model.ModelEvaluatorConfig{Context: config.Context, Globals: config.Globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: config.Cooperation, Trace: config.Trace})
		if e != nil {
			return result, false, errors.Join(e, md.DecRef())
		}
		current, step, e := advanceExprQEScalarTree(root, variables, context, eval, config, rw, nnf, solver, false)
		if e == nil && step == exprQEChooseVariable {
			var outcome exprQEPopStatus
			_, outcome, e = popExprQEBoolTree(current, variables, context, plugin, partitioner, solver, eval, md, config, rw, nnf, &free, false)
			if e == nil && (outcome == exprQEPopNeedsPartition || outcome == exprQEPopNoBranch) {
				e = errors.Join(eval.Close(), md.DecRef())
				return result, false, e
			}
		}
		e = errors.Join(e, eval.Close(), md.DecRef())
		if e != nil {
			return result, false, e
		}
	}
	if len(free) != 0 {
		return result, false, nil
	}
	leaves, err := root.leaves()
	if err != nil {
		return result, false, err
	}
	defer func() {
		for i := range leaves {
			err = errors.Join(err, m.DelRef(&leaves[i]))
		}
	}()
	forms := make([]ast.ASTHandle, len(leaves))
	for i, ref := range leaves {
		forms[i], err = m.RefGet(ref)
		if err != nil {
			return result, false, err
		}
	}
	formula, err := m.MkOrArgs(forms)
	if err != nil {
		return result, false, err
	}
	result, err = m.NewRef(formula)
	if err != nil {
		return result, false, err
	}
	if err = rw.Rewrite(formula, result); err != nil {
		return result, false, err
	}
	return result, true, nil
}
