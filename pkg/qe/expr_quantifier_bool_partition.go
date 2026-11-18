package qe

// Source: src/qe/qe.cpp, process_partition for scalar variables.

import (
	"errors"
	"io"

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
	// TRACE("qe", tout << fml << " free: " << m_current->free_vars() <<
	// "\n";) (AUDIT.md qe_05-F6).
	if err = qeTrace(m, config, "process_partition", "src/qe/qe.cpp", 1956, value, " free: ", func(out io.Writer) error { return writeExpressionVector(m, config, out, node.variables) }, "\n"); err != nil {
		return false, err
	}
	child, err := node.addUnit(config, value)
	if err != nil {
		return false, err
	}
	if err = child.resetFreeVars(); err != nil {
		return false, err
	}
	// block_assignment() walks from m_current, which process_partition does
	// not advance to the new child (AUDIT.md qe_02-F3).
	if err = assertExprQETreeConstraint(node, solver, true); err != nil {
		return false, err
	}
	return true, nil
}
