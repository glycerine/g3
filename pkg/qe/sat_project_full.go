package qe

// Source: src/qe/qe_sat_tactic.cpp, solver_context::project_var_full.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// projectFull is solver_context::project_var_full.
func (c *satSolverContext) projectFull(index uint32, rw *rewriter.TheoryRewriter, config Config) (err error) {
	if c == nil || c.closed || rw == nil {
		return ast.ErrASTHandle
	}
	if index >= uint32(len(c.variables)) {
		return arena.ErrBounds
	}
	m := c.manager
	variable, err := m.RefGet(c.variables[index])
	if err != nil {
		return err
	}
	formula, err := m.RefGet(c.formula)
	if err != nil {
		return err
	}
	abstractor, err := ast.NewExprAbstractor(m)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, abstractor.Close()) }()
	quantified, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&quantified)) }()
	if err = abstractor.MkExists([]ast.ASTHandle{variable}, formula, quantified); err != nil {
		return err
	}
	q, err := m.RefGet(quantified)
	if err != nil {
		return err
	}
	// expr_quant_elim qelim(m, m_fparams); qelim(m.mk_true(), m_fml, result)
	truth, err := m.MkTrue()
	if err != nil {
		return err
	}
	qelim := newExprQuantElim(m, config)
	projected, err := qelim.run(truth, q)
	err = errors.Join(err, qelim.Close())
	if err != nil {
		if projected != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&projected))
		}
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&projected)) }()
	value, err := m.RefGet(projected)
	if err != nil {
		return err
	}
	if err = rw.Rewrite(value, projected); err != nil {
		return err
	}
	value, err = m.RefGet(projected)
	if err != nil {
		return err
	}
	return c.eliminateVariable(index, value, ast.ASTHandle{})
}
