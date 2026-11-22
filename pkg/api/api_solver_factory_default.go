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

// NewSolverForIncrementalChecks constructs a strategic solver for callers that
// normally push before checking. Incremental checks can use the SMT backend
// with the default tactic portfolio still unbuilt. A tactic check, fallback,
// translation or full parameter query constructs it on demand. The tactic
// constructors read global settings and charge their memory at that point.
// Debugging, tracing and allocation-failure injection use ordinary initialization.
func (c *SMTContext) NewSolverForIncrementalChecks() (*APISimpleSolver, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	return c.saveSolver(newLazyAPISolver(c.smtContextState, c.command.NewIncrementalStrategicSolver))
}
