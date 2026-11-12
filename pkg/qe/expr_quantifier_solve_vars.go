package qe

// Source: src/qe/qe.cpp, quant_elim_plugin::solve_vars.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// exprQESolvingPlugin is a plugin whose qe_solver_plugin::solve is
// overridden with conj_enum and formula arguments.
type exprQESolvingPlugin interface {
	Solve(conjs *ConjunctionEnum, formula ast.ASTHandle) (bool, error)
}

// solveExprQEVars is quant_elim_plugin::solve_vars: every plugin, in family
// order, may solve a variable of the current node; any success restarts the
// pass on the new current node. The context is bound to the tree for the
// duration, so plugin callbacks reach the current node.
func solveExprQEVars(tree *exprQETreeContext, context *SolverContext) (err error) {
	if tree == nil || tree.current == nil || context == nil {
		return ast.ErrASTHandle
	}
	if err = context.open(); err != nil {
		return err
	}
	if context.tree != nil {
		return ast.ErrASTDomain
	}
	context.tree = tree
	defer func() { context.tree = nil }()
	m := tree.current.manager
	for {
		formula, err := m.RefGet(tree.current.formula)
		if err != nil {
			return err
		}
		conjs, err := NewConjunctionEnum(m, formula)
		if err != nil {
			return err
		}
		solved := false
		for _, plugin := range context.plugins {
			if plugin == nil {
				continue
			}
			switch p := plugin.(type) {
			case *BoolPlugin:
				solved, err = p.SolveContext(conjs, formula, tree.current.atoms)
			case exprQESolvingPlugin:
				solved, err = p.Solve(conjs, formula)
			}
			if err != nil || solved {
				break
			}
		}
		if err = errors.Join(err, conjs.Close()); err != nil {
			return err
		}
		if !solved {
			return nil
		}
	}
}
