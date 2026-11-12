package qe

// Source: src/qe/qe_sat_tactic.cpp, solver_context::project_var_partial.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/messages"
)

// projectPartial removes one variable using the current model and publishes
// the normalized formula and updated atom index in the concrete SAT context.
func (c *satSolverContext) projectPartial(index uint32, source *model.Model, rw *rewriter.TheoryRewriter, config Config) (err error) {
	if c == nil || c.closed || source == nil || rw == nil {
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
	sort, err := m.GetSort(variable)
	if err != nil {
		return err
	}
	family, err := m.GetFamilyID(sort)
	if err != nil {
		return err
	}
	arithmetic, err := m.ArithmeticFamilyID()
	if err != nil {
		return err
	}
	bv, err := m.BitVectorUtilityFamilyID()
	if err != nil {
		return err
	}
	var projected bool
	switch family {
	case m.BasicFamilyID():
		if err = c.boolean.Project(c.contains[index], source, c.formula, config); err != nil {
			return err
		}
		projected = true
	case arithmetic:
		projected, err = c.arithmetic.ProjectModelFormulaNNF(c.contains[index], source, c.formula, config)
		if err == nil && projected {
			err = c.takePartialArithmeticOutputs(c.arithmetic)
		}
	case bv:
		projected, err = c.bv.ProjectModel(c.contains[index], source, c.formula, config)
	default:
		return tactic.NewTacticException(messages.Canceled) // check_success
	}
	if err != nil {
		return err
	}
	if !projected {
		return tactic.NewTacticException(messages.Canceled) // check_success
	}
	current, err := c.manager.RefGet(c.formula)
	if err != nil {
		return err
	}
	if err = rw.Rewrite(current, c.formula); err != nil {
		return err
	}
	current, err = c.manager.RefGet(c.formula)
	if err != nil {
		return err
	}
	return c.eliminateVariable(index, current, ast.ASTHandle{})
}
