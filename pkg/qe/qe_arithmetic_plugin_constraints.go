package qe

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// ArithmeticConstraint is one i_solver_context::add_constraint(use_var,
// l1, l2) call kept for a host that drains it later.
type ArithmeticConstraint struct {
	UseVariable bool
	Formula     ast.ASTRef
	// Second is the optional second literal l2.
	Second ast.ASTRef
}

// literals returns the constraint's literals as borrowed handles.
func (c ArithmeticConstraint) literals(m *ast.ASTManager) ([]ast.ASTHandle, error) {
	h, err := m.RefGet(c.Formula)
	if err != nil {
		return nil, err
	}
	result := []ast.ASTHandle{h}
	if c.Second != (ast.ASTRef{}) {
		if h, err = m.RefGet(c.Second); err != nil {
			return nil, err
		}
		result = append(result, h)
	}
	return result, nil
}

// release drops the constraint's references.
func (c *ArithmeticConstraint) release(m *ast.ASTManager) (err error) {
	if c.Formula != (ast.ASTRef{}) {
		err = m.DelRef(&c.Formula)
	}
	if c.Second != (ast.ASTRef{}) {
		err = errors.Join(err, m.DelRef(&c.Second))
	}
	return err
}

func (p *ArithmeticPlugin) appendConstraint(useVariable bool, formula ast.ASTHandle) error {
	return p.appendConstraint2(useVariable, formula, ast.ASTHandle{})
}

// appendConstraint2 is m_ctx.add_constraint(use_var, l1, l2); a null l2 is
// omitted.
func (p *ArithmeticPlugin) appendConstraint2(useVariable bool, first, second ast.ASTHandle) error {
	if p.context.sink != nil {
		lits := []ast.ASTHandle{first}
		if !second.IsNull() {
			lits = append(lits, second)
		}
		return assertExprQETreeConstraint(p.context.sink.current, p.context.sink.solver, useVariable, lits...)
	}
	c := ArithmeticConstraint{UseVariable: useVariable}
	var err error
	if c.Formula, err = p.manager.NewRef(first); err != nil {
		return err
	}
	if !second.IsNull() {
		if c.Second, err = p.manager.NewRef(second); err != nil {
			return errors.Join(err, c.release(p.manager))
		}
	}
	p.constraints = append(p.constraints, c)
	return nil
}

func (p *ArithmeticPlugin) clearConstraints() (result error) {
	for i := len(p.constraints) - 1; i >= 0; i-- {
		result = errors.Join(result, p.constraints[i].release(p.manager))
	}
	p.constraints = nil
	return result
}

func (p *ArithmeticPlugin) TakeConstraints() ([]ArithmeticConstraint, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	result := p.constraints
	p.constraints = nil
	return result, nil
}
