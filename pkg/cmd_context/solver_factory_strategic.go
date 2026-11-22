package cmd_context

// Source: src/api/api_solver.cpp:76-104.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/tactic/portfolio"
)

func (c *Context) NewStrategicSolver(logic string, requireRecognized bool) (solver.Solver, error) {
	if requireRecognized && !solver.SupportedLogic(logic) {
		return nil, fmt.Errorf("logic '%s' is not recognized", logic)
	}
	factory, err := portfolio.NewSMTStrategicSolverFactory(logic, c.config)
	if err != nil {
		return nil, err
	}
	return c.newSolverFromFactory(factory, logic)
}

// NewIncrementalStrategicSolver uses the incremental factory for verification
// clients that create a solver for each batch of scoped checks.
func (c *Context) NewIncrementalStrategicSolver(logic string) (solver.Solver, error) {
	factory, err := portfolio.NewIncrementalStrategicSolverFactory(logic, c.config)
	if err != nil {
		return nil, err
	}
	return c.newSolverFromFactory(factory, logic)
}
