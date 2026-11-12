package qe

// Source: src/qe/qe_sat_tactic.cpp, default NNF callbacks and the concrete
// solver_context formula initialization and constraint assertion.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
)

type satNNFDefault struct{ manager *ast.ASTManager }

func (d *satNNFDefault) IsRelevant(ast.ASTHandle) (bool, error) { return true, nil }
func (d *satNNFDefault) NormalizeAtom(h ast.ASTHandle, positive bool) (ast.ASTHandle, error) {
	if positive {
		return h, nil
	}
	return d.manager.MkNot(h)
}

// satSolverContext is sat_tactic::solver_context. It borrows the enclosing
// tactic's SMT solver. Its formula, atom sets and the plugins that project()
// registers (arith, bool, bv, in that order) are owned for its lifetime.
type satSolverContext struct {
	manager *ast.ASTManager
	solver  *smtcore.SMTSolver
	// plugins is i_solver_context's plugin table.
	plugins    *SolverContext
	arithmetic *ArithmeticPlugin
	boolean    *BoolPlugin
	bv         *BitVectorPlugin
	normalizer *QENNFNormalizer
	formula    ast.ASTRef
	atoms      *BoolAtomIndex
	variables  []ast.ASTRef
	contains   []*ast.ContainsApp
	closed     bool
}

// satSolverContextNNF is i_solver_context's get_is_relevant and get_mk_atom
// over the solver_context: relevance asks each contains_app in index order
// and atoms go to the plugins in family order.
type satSolverContextNNF struct{ c *satSolverContext }

func (a *satSolverContextNNF) IsRelevant(expression ast.ASTHandle) (bool, error) {
	for _, contains := range a.c.contains {
		found, err := contains.Check(expression)
		if err != nil || found {
			return found, err
		}
	}
	return false, nil
}

func (a *satSolverContextNNF) NormalizeAtom(ast.ASTHandle, bool) (ast.ASTHandle, error) {
	return ast.ASTHandle{}, ast.ErrASTDomain
}

// NormalizeAtomRef is i_solver_context::mk_atom. Only the arithmetic plugin
// overrides qe_solver_plugin::mk_atom among the registered plugins.
func (a *satSolverContextNNF) NormalizeAtomRef(expression ast.ASTHandle, positive bool) (ast.ASTRef, error) {
	m := a.c.manager
	ref, handled, err := a.c.arithmetic.mkAtomRef(expression, positive)
	if err != nil || handled {
		return ref, err
	}
	if !positive {
		if expression, err = m.MkNotSimplified(expression); err != nil {
			return ast.ASTRef{}, err
		}
	}
	return m.NewRef(expression)
}

func newSATSolverContext(manager *ast.ASTManager, solver *smtcore.SMTSolver, config Config) (_ *satSolverContext, result error) {
	if manager == nil || solver == nil {
		return nil, ast.ErrASTHandle
	}
	owner, err := solver.Manager()
	if err != nil {
		return nil, err
	}
	if owner != manager {
		return nil, ast.ErrASTHandle
	}
	c := &satSolverContext{manager: manager, solver: solver}
	defer func() {
		if result != nil {
			result = errors.Join(result, c.Close())
		}
	}()
	c.formula, err = manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, err
	}
	if c.plugins, err = NewSolverContext(manager, nil); err != nil {
		return nil, err
	}
	arithmetic, err := NewArithmeticPlugin(c.plugins, config)
	if err != nil {
		return nil, err
	}
	if err = c.plugins.AddPlugin(arithmetic); err != nil {
		return nil, errors.Join(err, arithmetic.Close())
	}
	c.arithmetic = arithmetic
	boolean, err := NewBoolPlugin(c.plugins)
	if err != nil {
		return nil, err
	}
	if err = c.plugins.AddPlugin(boolean); err != nil {
		return nil, errors.Join(err, boolean.Close())
	}
	c.boolean = boolean
	bv, err := NewBitVectorPlugin(c.plugins)
	if err != nil {
		return nil, err
	}
	if err = c.plugins.AddPlugin(bv); err != nil {
		return nil, errors.Join(err, bv.Close())
	}
	c.bv = bv
	nnf := &satSolverContextNNF{c: c}
	if c.normalizer, err = NewQENNFNormalizer(manager, nnf, nnf, config); err != nil {
		return nil, err
	}
	return c, nil
}

// initializeFormula is the m_fml assignment, atom-set reset and get_nnf of
// init and elim_var.
func (c *satSolverContext) initializeFormula(formula ast.ASTHandle) (result error) {
	if c == nil || c.closed {
		return ast.ErrASTHandle
	}
	if err := c.manager.SetRef(c.formula, formula); err != nil {
		return err
	}
	if c.atoms != nil {
		if err := c.atoms.Close(); err != nil {
			return err
		}
		c.atoms = nil
	}
	atoms, err := c.normalizer.Normalize(c.formula)
	if err != nil {
		return err
	}
	c.atoms = atoms
	return nil
}

// addVariable stores both the application and the source contains-app helper.
func (c *satSolverContext) addVariable(variable ast.ASTHandle) error {
	if c == nil || c.closed {
		return ast.ErrASTHandle
	}
	contains, err := ast.NewContainsApp(c.manager, variable)
	if err != nil {
		return err
	}
	ref, err := c.manager.NewRef(variable)
	if err != nil {
		return errors.Join(err, contains.Close())
	}
	c.variables = append(c.variables, ref)
	c.contains = append(c.contains, contains)
	return nil
}

func (c *satSolverContext) variable(index uint32) (ast.ASTHandle, error) {
	if c == nil || c.closed {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	if index >= uint32(len(c.variables)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return c.manager.RefGet(c.variables[index])
}

func (c *satSolverContext) isVariable(expr ast.ASTHandle) (uint32, bool, error) {
	if c == nil || c.closed {
		return 0, false, ast.ErrASTHandle
	}
	for i := range c.variables {
		h, err := c.manager.RefGet(c.variables[i])
		if err != nil {
			return 0, false, err
		}
		if h == expr {
			return uint32(i), true, nil
		}
	}
	return 0, false, nil
}

// eliminateVariable recomputes NNF then erases the selected entry while
// preserving the order of the remaining variables. The definition is unused
// by the source solver context.
func (c *satSolverContext) eliminateVariable(index uint32, formula, definition ast.ASTHandle) error {
	if c == nil || c.closed {
		return ast.ErrASTHandle
	}
	if index >= uint32(len(c.variables)) {
		return arena.ErrBounds
	}
	if err := c.initializeFormula(formula); err != nil {
		return err
	}
	if c.contains[index] != nil {
		if err := c.contains[index].Close(); err != nil {
			return err
		}
		c.contains[index] = nil
	}
	if err := c.manager.DelRef(&c.variables[index]); err != nil {
		return err
	}
	copy(c.variables[index:], c.variables[index+1:])
	c.variables = c.variables[:len(c.variables)-1]
	copy(c.contains[index:], c.contains[index+1:])
	c.contains = c.contains[:len(c.contains)-1]
	return nil
}

// addConstraint intentionally ignores useVar, matching the source callback.
func (c *satSolverContext) addConstraint(useVar bool, literals ...ast.ASTHandle) error {
	if c == nil || c.closed {
		return ast.ErrASTHandle
	}
	if len(literals) > 3 {
		return ast.ErrASTDomain
	}
	args := make([]ast.ASTHandle, 0, len(literals))
	for _, h := range literals {
		if !h.IsNull() {
			args = append(args, h)
		}
	}
	clause, err := c.manager.MkOrArgs(args)
	if err != nil {
		return err
	}
	ref, err := c.manager.NewRef(clause)
	if err != nil {
		return err
	}
	err = c.solver.AssertFormula(clause)
	return errors.Join(err, c.manager.DelRef(&ref))
}

// Close is ~solver_context: the contains_app objects in order, then the
// members in reverse declaration order (m_fml, m_vars, the atom sets), then
// i_solver_context's plugins in ascending family order.
func (c *satSolverContext) Close() (result error) {
	if c == nil || c.closed {
		return nil
	}
	for i, contains := range c.contains {
		if contains != nil {
			if err := contains.Close(); err != nil {
				return err
			}
			c.contains[i] = nil
		}
	}
	c.contains = nil
	if c.formula != (ast.ASTRef{}) {
		if err := c.manager.DelRef(&c.formula); err != nil {
			return err
		}
	}
	for i := range c.variables {
		if c.variables[i] != (ast.ASTRef{}) {
			if err := c.manager.DelRef(&c.variables[i]); err != nil {
				return err
			}
		}
	}
	c.variables = nil
	if c.atoms != nil {
		if err := c.atoms.Close(); err != nil {
			return err
		}
		c.atoms = nil
	}
	if c.normalizer != nil {
		if err := c.normalizer.Close(); err != nil {
			return err
		}
		c.normalizer = nil
	}
	if c.plugins != nil {
		if err := c.plugins.Close(); err != nil {
			return err
		}
		c.plugins = nil
	}
	c.closed = true
	return nil
}
