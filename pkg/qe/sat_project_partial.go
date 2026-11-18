package qe

// Source: src/qe/qe_sat_tactic.cpp, solver_context::project_var_partial.

import (
	"io"

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
		// arith_plugin::project's update_bounds walks the context's
		// pos_atoms()/neg_atoms() in obj_hashtable order (AUDIT.md
		// qe_01-F6).
		var positive, negative []ast.ASTHandle
		if positive, err = c.atoms.positiveTable(); err != nil {
			return err
		}
		if negative, err = c.atoms.negativeTable(); err != nil {
			return err
		}
		projected, err = c.arithmetic.ProjectModelFormulaTables(c.contains[index], source, c.formula, positive, negative, config)
		if err == nil && projected {
			err = c.takePartialArithmeticOutputs(c.arithmetic)
		}
	case bv:
		projected, err = c.bv.ProjectModel(c.contains[index], source, c.formula, config)
	default:
		return satCheckFailed(m, config) // check_success(has_plugin(x))
	}
	if err != nil {
		return err
	}
	if !projected {
		return satCheckFailed(m, config) // check_success(p.project(...))
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
	// TRACE("qe", model_v2_pp(tout, *model); tout << "\n";
	//       tout << mk_pp(m_fml, m) << "\n";) (AUDIT.md qe_05-F6).
	showModel := func(out io.Writer) error { return source.Core().WriteV2(out, config.Context, config.Globals, false) }
	if err = qeTrace(m, config, "project_var_partial", "src/qe/qe_sat_tactic.cpp", 180, showModel, "\n", current, "\n"); err != nil {
		return err
	}
	return c.eliminateVariable(index, current, ast.ASTHandle{})
}

// satCheckFailed is sat_tactic::check_success on failure: TRACE("qe",
// tout << "check failed\n";) then the canceled tactic exception.
func satCheckFailed(m *ast.ASTManager, config Config) error {
	if err := qeTrace(m, config, "check_success", "src/qe/qe_sat_tactic.cpp", 678, "check failed\n"); err != nil {
		return err
	}
	return tactic.NewTacticException(messages.Canceled)
}
