package api

// Source: src/api/api_solver.cpp:87-104 (Z3_mk_solver_for_logic checks the
// logic at once; the factory fixes it, and init_solver builds the solver).

import (
	"fmt"

	"github.com/glycerine/g3/pkg/solver"
)

func (c *SMTContext) NewSolverForLogic(logic string) (*APISimpleSolver, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if !solver.SupportedLogic(logic) {
		return nil, fmt.Errorf("logic %q is not recognized", logic)
	}
	command := c.command
	return c.saveSolver(newLazyAPISolver(c.smtContextState, func(string) (solver.Solver, error) {
		return command.NewStrategicSolver(logic, true)
	}))
}
