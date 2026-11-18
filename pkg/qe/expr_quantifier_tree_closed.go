package qe

// Source: src/qe/qe.cpp, pop's satisfiable closed-partition case.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
)

// closeExprQETreePartition handles a satisfiable node after partition analysis
// finds no mixed conjuncts and no independent groups. The caller must establish
// satisfiability; the resulting child contains the closed part of the formula.
func closeExprQETreePartition(node *exprQETree, part *exprQETreePartition, solver *smtcore.SMTSolver, evaluator *model.ModelEvaluator, modelCore *model.ModelCore, config Config, withDefinitions bool) (child *exprQETree, handled bool, err error) {
	if node == nil || node.manager == nil || node.formula == (ast.ASTRef{}) || node.selected != (ast.ASTRef{}) || len(node.children) != 0 || part == nil || part.manager != node.manager || solver == nil || (withDefinitions && evaluator == nil) {
		return nil, false, ast.ErrASTHandle
	}
	m := node.manager
	mixed, err := m.RefGet(part.mixed)
	if err != nil {
		return nil, false, err
	}
	if part.split || !m.IsTrue(mixed) {
		return nil, false, nil
	}
	closed, err := m.RefGet(part.closed)
	if err != nil {
		return nil, false, err
	}
	child, err = node.addUnit(config, closed)
	if err != nil {
		return nil, false, err
	}
	var completion *model.ModelEvaluator
	defer func() {
		if completion != nil {
			err = errors.Join(err, completion.Close())
		}
	}()
	for _, ref := range child.variables {
		if !withDefinitions {
			break
		}
		variable, e := m.RefGet(ref)
		if e != nil {
			return nil, false, e
		}
		value, e := evaluator.EvaluateValue(variable)
		if e != nil {
			return nil, false, e
		}
		h, e := m.RefGet(value)
		if e == nil && h == variable {
			if completion == nil {
				if modelCore == nil {
					e = ast.ErrASTHandle
				} else {
					completion, e = model.NewModelEvaluator(modelCore, model.ModelEvaluatorConfig{Context: config.Context, Globals: config.Globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: config.Cooperation, Trace: config.Trace})
					if e == nil {
						e = completion.SetModelCompletion(true)
					}
				}
			}
			if e == nil {
				e = completion.Evaluate(variable, value)
			}
			if e == nil {
				h, e = m.RefGet(value)
			}
		}
		if e == nil {
			// TRACE("qe", tout << mk_pp(x,m) << " " << mk_pp(val, m) << "\n";)
			// (AUDIT.md qe_05-F6).
			e = qeTrace(m, config, "update_current", "src/qe/qe.cpp", 1780, variable, " ", h, "\n")
		}
		if e == nil {
			e = child.addDefinition(variable, h)
		}
		e = errors.Join(e, m.DelRef(&value))
		if e != nil {
			return nil, false, e
		}
	}
	if err = child.resetFreeVars(); err != nil {
		return nil, false, err
	}
	if err = assertExprQETreeConstraint(child, solver, true); err != nil {
		return nil, false, err
	}
	return child, true, nil
}
