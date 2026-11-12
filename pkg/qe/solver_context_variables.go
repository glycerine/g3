package qe

// Source: src/qe/qe.cpp:2541-2544 direct blast_or context add_var.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// AddVariable appends a borrowed application exactly as the direct source
// context appends to its caller-owned app_ref_vector.
func (c *SolverContext) AddVariable(variable ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	kind, err := c.manager.GetKind(variable)
	if err != nil || kind != ast.ASTApp {
		return errors.Join(ast.ErrASTDomain, err)
	}
	if c.tree != nil {
		// quant_elim_plugin::add_var: m_new_vars and the branch tables.
		return c.tree.variables.add(variable)
	}
	c.variables = append(c.variables, variable)
	c.variableOwners = append(c.variableOwners, ast.ASTRef{})
	c.contains = append(c.contains, nil)
	return nil
}

// adoptVariable transfers an existing reference into the context's variable
// vector. Ownership transfers only after the borrowed insertion succeeds.
func (c *SolverContext) adoptVariable(variable ast.ASTHandle, owner *ast.ASTRef) error {
	if err := c.open(); err != nil {
		return err
	}
	if owner == nil || *owner == (ast.ASTRef{}) {
		return ast.ErrASTHandle
	}
	h, err := c.manager.RefGet(*owner)
	if err != nil {
		return err
	}
	if h != variable {
		return ast.ErrASTDomain
	}
	if c.tree != nil {
		if err = c.AddVariable(variable); err != nil {
			return err
		}
		return c.manager.DelRef(owner)
	}
	if err = c.AddVariable(variable); err != nil {
		return err
	}
	c.variableOwners[len(c.variableOwners)-1] = *owner
	*owner = ast.ASTRef{}
	return nil
}

// Variables returns the current source order as a detached borrowed slice.
func (c *SolverContext) Variables() ([]ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if c.tree != nil {
		result := make([]ast.ASTHandle, len(c.tree.current.variables))
		for i := range result {
			var err error
			if result[i], err = c.tree.variable(uint32(i)); err != nil {
				return nil, err
			}
		}
		return result, nil
	}
	return append([]ast.ASTHandle(nil), c.variables...), nil
}

// resetVariables is the variable half of quant_elim_plugin::reset: the
// current node's free variables, their contains_app objects and the
// published formula are dropped; registered plugins stay.
func (c *SolverContext) resetVariables() error {
	if err := c.open(); err != nil {
		return err
	}
	if err := c.closeMutationState(); err != nil {
		return err
	}
	c.containsClosing = 0
	c.variables = nil
	c.variableOwners = nil
	return nil
}
