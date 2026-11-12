package qe

// Source: src/qe/qe.cpp, process_partition for scalar variables.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
)

// processExprQEScalarPartition is quant_elim_plugin::process_partition: each
// independent group goes through quant_elim_new::eliminate_exists in source
// reverse order, then the result is installed as a unit branch and blocked.
func processExprQEScalarPartition(node *exprQETree, partition *exprQETreePartition, context *SolverContext, solver *smtcore.SMTSolver, config Config) (complete bool, err error) {
	if node == nil || node.manager == nil || partition == nil || partition.manager != node.manager || !partition.split || solver == nil || context == nil || context.owner == nil {
		return false, ast.ErrASTHandle
	}
	m := node.manager
	original, err := m.RefGet(node.formula)
	if err != nil {
		return false, err
	}
	formula, err := m.NewRef(original)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, m.DelRef(&formula)) }()
	for {
		group, ok, e := partition.popVariables(node)
		if e != nil {
			return false, e
		}
		if !ok {
			break
		}
		// m_qe.eliminate_exists(vars, fml, m_free_vars, m_get_first, m_defs)
		th := context.owner
		if _, e = th.qe.eliminateExists(group, formula, &th.freeVars); e != nil {
			return false, e
		}
	}
	value, err := m.RefGet(formula)
	if err != nil {
		return false, err
	}
	child, err := node.addUnit(config, value)
	if err != nil {
		return false, err
	}
	for len(child.variables) > 0 {
		i := len(child.variables) - 1
		if err = m.DelRef(&child.variables[i]); err != nil {
			return false, err
		}
		child.variables = child.variables[:i]
	}
	if err = assertExprQETreeConstraint(child, solver, true); err != nil {
		return false, err
	}
	return true, nil
}
