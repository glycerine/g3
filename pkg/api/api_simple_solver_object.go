package api

// Source: src/api/api_solver.cpp:65-73,249-262; src/api/api_solver.h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/solver"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type APISimpleSolver struct {
	owner            *smtContextState
	solver           solver.Solver
	explicitContext  *uparams.ParamsContext
	explicitParams   uparams.ParamsRef
	explicitReady    bool
	pendingStrategic bool
	closed           bool
}

func (c *SMTContext) NewSimpleSolver() (*APISimpleSolver, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	value, err := c.command.NewSimpleSolver()
	if err != nil {
		return nil, err
	}
	return &APISimpleSolver{owner: c.smtContextState, solver: value}, nil
}

func (s *APISimpleSolver) validOwner() error {
	if s == nil || s.owner == nil || s.closed || s.owner.closed || s.owner.command == nil || (!s.pendingStrategic && s.solver == nil) {
		return ErrSMTContextOwner
	}
	return nil
}

func (s *APISimpleSolver) open() error {
	if err := s.validOwner(); err != nil {
		return err
	}
	if s.pendingStrategic {
		var explicit *uparams.ParamsRef
		if s.explicitReady {
			explicit = &s.explicitParams
		}
		value, err := s.owner.command.NewStrategicSolverWithParams("", false, explicit)
		if err != nil {
			return err
		}
		s.solver, s.pendingStrategic = value, false
		if explicit != nil {
			params := &ParameterSet{owner: s.owner, params: s.explicitContext, value: s.explicitParams}
			if err := s.applyParameters(params); err != nil {
				closeErr := value.Close()
				s.solver, s.pendingStrategic = nil, true
				return errors.Join(err, closeErr)
			}
		}
	}
	_, err := s.solver.Manager()
	return err
}

func (s *APISimpleSolver) Close() error {
	if s == nil || s.closed {
		return nil
	}
	if s.solver == nil && !s.pendingStrategic {
		return ErrSMTContextOwner
	}
	if s.explicitReady {
		if err := s.explicitContext.Del(&s.explicitParams); err != nil {
			return err
		}
		s.explicitReady = false
		s.explicitContext = nil
	}
	if s.solver != nil {
		if err := s.solver.Close(); err != nil {
			return err
		}
	}
	s.owner, s.solver = nil, nil
	s.closed = true
	return nil
}
