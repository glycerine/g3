package qe

// Source: src/qe/qe.cpp, simplify_solver_context::solve and simplify_exists.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

type qeSimplifyResult struct {
	manager   *ast.ASTManager
	formula   ast.ASTRef
	variables []ast.ASTRef
}

func (r *qeSimplifyResult) Close() error {
	if r == nil || r.manager == nil {
		return nil
	}
	for len(r.variables) != 0 {
		i := len(r.variables) - 1
		if err := r.manager.DelRef(&r.variables[i]); err != nil {
			return err
		}
		r.variables = r.variables[:i]
	}
	if r.formula != (ast.ASTRef{}) {
		if err := r.manager.DelRef(&r.formula); err != nil {
			return err
		}
	}
	r.manager = nil
	return nil
}

// simplifySolverContext is qe.cpp simplify_solver_context: its constructor
// registers the Boolean and arithmetic plugins, which live as long as the
// context, and solve re-initializes the variables for each call.
type simplifySolverContext struct {
	manager    *ast.ASTManager
	context    *SolverContext
	boolean    *BoolPlugin
	arithmetic *ArithmeticPlugin
	// m_pos and m_neg; the source context never fills them.
	atoms *BoolAtomIndex
}

func newSimplifySolverContext(m *ast.ASTManager, config Config) (result *simplifySolverContext, err error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	result = &simplifySolverContext{manager: m}
	defer func() {
		if err != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	if result.context, err = NewSolverContext(m, nil); err != nil {
		return result, err
	}
	if result.atoms, err = NewBoolAtomIndex(m, nil, nil); err != nil {
		return result, err
	}
	boolean, err := NewBoolPlugin(result.context)
	if err != nil {
		return result, err
	}
	if err = result.context.AddPlugin(boolean); err != nil {
		return result, errors.Join(err, boolean.Close())
	}
	result.boolean = boolean
	arithmetic, err := NewArithmeticPlugin(result.context, config)
	if err != nil {
		return result, err
	}
	if err = result.context.AddPlugin(arithmetic); err != nil {
		return result, errors.Join(err, arithmetic.Close())
	}
	result.arithmetic = arithmetic
	return result, nil
}

// Close is ~simplify_solver_context: reset releases the contains_app
// objects, then the members, then i_solver_context's plugins in ascending
// family order.
func (s *simplifySolverContext) Close() error {
	if s == nil || s.manager == nil {
		return nil
	}
	if s.context != nil {
		if err := s.context.resetVariables(); err != nil {
			return err
		}
	}
	if s.atoms != nil {
		if err := s.atoms.Close(); err != nil {
			return err
		}
		s.atoms = nil
	}
	if s.context != nil {
		if err := s.context.Close(); err != nil {
			return err
		}
		s.context = nil
	}
	s.boolean, s.arithmetic = nil, nil
	s.manager = nil
	return nil
}

// solve is simplify_solver_context::solve: it solves source Boolean units
// and arithmetic equations to a fixed point. Variables are borrowed; formula
// is the caller's expr_ref, updated in place. The remaining variables are
// returned as references the caller owns.
func (s *simplifySolverContext) solve(variables []ast.ASTHandle, formula ast.ASTRef) (remaining []ast.ASTRef, err error) {
	if s == nil || s.manager == nil || s.context == nil {
		return nil, ast.ErrASTHandle
	}
	m := s.manager
	defer func() {
		if err != nil {
			for i := range remaining {
				err = errors.Join(err, m.DelRef(&remaining[i]))
			}
			remaining = nil
		}
	}()
	context := s.context
	// init: reset the contains_app objects and borrow the caller's vector.
	if err = context.resetVariables(); err != nil {
		return nil, err
	}
	for i, v := range variables {
		if err = context.AddVariable(v); err != nil {
			return nil, err
		}
		// init allocates each contains_app eagerly; they keep their
		// variables alive until the next init or the destructor.
		if _, err = context.Contains(uint32(i)); err != nil {
			return nil, err
		}
	}
	for {
		canceled, e := m.Canceled()
		if e != nil {
			return nil, e
		}
		if canceled {
			return nil, tactic.NewTacticException(m.ResourceLimit().CancelMessage())
		}
		f, e := m.RefGet(formula)
		if e != nil {
			return nil, e
		}
		conjs, e := NewConjunctionEnum(m, f)
		if e != nil {
			return nil, e
		}
		solved, e := s.boolean.SolveContext(conjs, f, s.atoms)
		if e == nil && !solved {
			solved, e = s.arithmetic.Solve(conjs, f)
		}
		if e != nil {
			return nil, errors.Join(e, conjs.Close())
		}
		if solved {
			// elim_var assigned *m_fml during the plugin's solve.
			f, ok, e := context.CurrentFormula()
			if e == nil && !ok {
				e = ast.ErrASTDomain
			}
			if e == nil {
				e = m.SetRef(formula, f)
			}
			if e == nil {
				e = context.releaseCurrentFormula()
			}
			if e != nil {
				return nil, errors.Join(e, conjs.Close())
			}
		}
		if e = conjs.Close(); e != nil {
			return nil, e
		}
		if !solved {
			break
		}
	}
	current, err := context.Variables()
	if err != nil {
		return nil, err
	}
	for _, v := range current {
		ref, e := m.NewRef(v)
		if e != nil {
			return remaining, e
		}
		remaining = append(remaining, ref)
	}
	return remaining, nil
}
