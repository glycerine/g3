package api

// Source: src/api/api_solver.cpp:76-85 (Z3_mk_solver: the handle holds
// mk_smt_strategic_solver_factory(); init_solver builds the solver).

import "github.com/glycerine/g3/pkg/solver"

func (c *SMTContext) NewSolver() (*APISimpleSolver, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	command := c.command
	return c.saveSolver(newLazyAPISolver(c.smtContextState, func(logic string) (solver.Solver, error) {
		return command.NewStrategicSolver(logic, false)
	}))
}
