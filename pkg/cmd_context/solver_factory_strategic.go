package cmd_context

// Source: src/api/api_solver.cpp:76-104.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/tactic/portfolio"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (c *Context) NewStrategicSolver(logic string, requireRecognized bool) (solver.Solver, error) {
	return c.NewStrategicSolverWithParams(logic, requireRecognized, nil)
}

func (c *Context) NewStrategicSolverWithParams(logic string, requireRecognized bool, p *uparams.ParamsRef) (solver.Solver, error) {
	if requireRecognized && !solver.SupportedLogic(logic) {
		return nil, fmt.Errorf("logic %q is not recognized", logic)
	}
	factory, err := portfolio.NewSMTStrategicSolverFactory(logic, c.config)
	if err != nil {
		return nil, err
	}
	return c.newSolverFromFactory(factory, logic, p)
}
