package cmd_context

// Source: src/api/api_solver.cpp init_solver_core.

import (
	"errors"

	"github.com/glycerine/g3/pkg/solver"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (c *Context) newSolverFromFactory(factory solver.Factory, logic string, explicit *uparams.ParamsRef) (result solver.Solver, resultErr error) {
	if err := c.valid(); err != nil {
		return nil, err
	}
	if factory == nil {
		return nil, errors.New("command context: nil solver factory")
	}
	defer func() {
		resultErr = errors.Join(resultErr, factory.Close())
		if resultErr != nil && result != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	manager, err := c.Manager()
	if err != nil {
		return nil, err
	}
	params := &c.params
	if explicit != nil {
		local, err := c.config.Params.NewValue()
		if err != nil {
			return nil, err
		}
		defer func() { resultErr = errors.Join(resultErr, c.config.Params.Del(&local)) }()
		// Solver parameters override the context defaults for this backend.
		if err = c.config.Params.Assign(&local, &c.params); err != nil {
			return nil, err
		}
		if err = c.config.Params.Append(&local, explicit); err != nil {
			return nil, err
		}
		params = &local
	}
	var proofs, models, cores bool
	if err = c.parameters.GetSolverParams(manager, params, &proofs, &models, &cores); err != nil {
		return nil, err
	}
	return factory.New(manager, params, proofs, models, cores, logic)
}
