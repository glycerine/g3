package api

// Source: src/api/api_solver.cpp:106-115 (Z3_mk_solver_from_tactic: the
// tactic2solver factory holds a tactic_ref; init_solver builds the solver).

import "github.com/glycerine/g3/pkg/solver"

func (c *SMTContext) NewSolverFromTactic(value *APITactic) (*APISimpleSolver, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if err := value.open(); err != nil {
		return nil, err
	}
	if value.owner != c.smtContextState {
		return nil, ErrSMTContextOwner
	}
	held := value.tactic
	if err := held.IncRef(); err != nil {
		return nil, err
	}
	command := c.command
	s := newLazyAPISolver(c.smtContextState, func(string) (solver.Solver, error) {
		return command.NewSolverFromTactic(held)
	})
	s.release = held.DecRef
	return c.saveSolver(s)
}
