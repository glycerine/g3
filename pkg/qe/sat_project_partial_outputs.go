package qe

// Source: src/qe/qe_sat_tactic.cpp, solver_context add_var/add_constraint.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *satSolverContext) takePartialArithmeticOutputs(plugin *ArithmeticPlugin) (err error) {
	if c == nil || c.closed || plugin == nil {
		return ast.ErrASTHandle
	}
	m := c.manager
	constraints, err := plugin.TakeConstraints()
	if err != nil {
		return err
	}
	defer func() {
		for i := range constraints {
			err = errors.Join(err, constraints[i].release(m))
		}
	}()
	for _, constraint := range constraints {
		lits, e := constraint.literals(m)
		if e != nil {
			return e
		}
		if e = c.addConstraint(constraint.UseVariable, lits...); e != nil {
			return e
		}
	}
	fresh, err := plugin.TakeFreshVariables()
	if err != nil {
		return err
	}
	defer func() {
		for i := range fresh {
			if fresh[i] != (ast.ASTRef{}) {
				err = errors.Join(err, m.DelRef(&fresh[i]))
			}
		}
	}()
	for _, ref := range fresh {
		h, e := m.RefGet(ref)
		if e != nil {
			return e
		}
		if e = c.addVariable(h); e != nil {
			return e
		}
	}
	return nil
}
