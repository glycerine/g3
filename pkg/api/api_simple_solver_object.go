package api

// Source: src/api/api_solver.cpp:46-73,249-262; src/api/api_solver.h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/solver"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// APISimpleSolver is Z3_solver_ref: a solver factory, the parameters set
// on the handle (m_params), its logic (m_logic) and the solver itself
// (m_solver), which init_solver builds on first use. Creating the handle
// builds nothing: the ASTs a solver makes appear when it is first used,
// after the caller's own terms, as in z3.
type APISimpleSolver struct {
	owner           *smtContextState
	solver          solver.Solver
	create          func(logic string) (solver.Solver, error) // m_solver_factory
	release         func() error                              // drops the factory's references
	logic           string                                    // m_logic
	explicitContext *uparams.ParamsContext
	explicitParams  uparams.ParamsRef
	explicitReady   bool
	closed          bool
	last            apiLastHold // m_last_obj's reference (api_last_object.go)
}

// ErrSolverNoFactory is FORK FIX #122's default_exception: a solver made
// by Z3_solver_translate cannot be rebuilt after Z3_solver_reset.
var ErrSolverNoFactory = errors.New("solver was created by Z3_solver_translate and has no factory: it cannot be rebuilt after Z3_solver_reset")

func newLazyAPISolver(owner *smtContextState, create func(string) (solver.Solver, error)) *APISimpleSolver {
	return &APISimpleSolver{owner: owner, create: create}
}

func (c *SMTContext) NewSimpleSolver() (*APISimpleSolver, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	command := c.command
	return c.saveSolver(newLazyAPISolver(c.smtContextState, func(string) (solver.Solver, error) {
		return command.NewSimpleSolver()
	}))
}

// saveSolver is save_object of a new Z3_solver_ref.
func (c *smtContextState) saveSolver(s *APISimpleSolver) (*APISimpleSolver, error) {
	if err := c.saveObject(s, &s.last); err != nil {
		s.last.closed = true
		return nil, err
	}
	return s, nil
}

func (s *APISimpleSolver) lastRelease() error { return s.last.release(s.destroy) }

func (s *APISimpleSolver) open() error {
	if s == nil || s.owner == nil || s.closed || s.last.closed || s.owner.closed || s.owner.command == nil {
		return ErrSMTContextOwner
	}
	if s.solver == nil {
		return nil // init_solver builds it (or reports FORK FIX #122)
	}
	_, err := s.solver.Manager()
	return err
}

// use is open followed by init_solver.
func (s *APISimpleSolver) use() error {
	if err := s.open(); err != nil {
		return err
	}
	return s.ensure()
}

// ensure is init_solver / init_solver_core: build the solver from the
// factory, then validate the handle's parameters against its descriptions
// and apply them.
func (s *APISimpleSolver) ensure() (resultErr error) {
	if s.solver != nil {
		return nil
	}
	if s.create == nil {
		// FORK FIX #122: a translated handle has no factory; z3 4.7.1
		// called through the null factory after Z3_solver_reset.
		return ErrSolverNoFactory
	}
	value, err := s.create(s.logic)
	if err != nil {
		return err
	}
	s.solver = value
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, s.dropSolver())
		}
	}()
	if !s.explicitReady {
		return nil
	}
	view := &ParameterSet{owner: s.owner, params: s.explicitContext, value: s.explicitParams}
	descriptions, err := s.collectDescriptions()
	if err != nil {
		return err
	}
	if err = view.Validate(descriptions); err != nil {
		return errors.Join(err, descriptions.Close())
	}
	if err = descriptions.Close(); err != nil {
		return err
	}
	name := "model"
	models, err := s.explicitContext.GetBoolName(&s.explicitParams, &name, true)
	if err != nil {
		return err
	}
	if !models {
		if err = s.solver.SetProduceModels(false); err != nil {
			return err
		}
	}
	return s.updateSolverParams(view)
}

// dropSolver is m_solver = nullptr.
func (s *APISimpleSolver) dropSolver() error {
	if s.solver == nil {
		return nil
	}
	err := s.solver.Close()
	s.solver = nil
	return err
}

// Reset is Z3_solver_reset: the solver is dropped and rebuilt with the
// handle's parameters on next use.
func (s *APISimpleSolver) Reset() error {
	if err := s.open(); err != nil {
		return err
	}
	return s.dropSolver()
}

// Close is Z3_solver_dec_ref of the handle's reference: the handle is
// unusable at once; the solver dies now, or when the context's last-object
// reference to it is released.
func (s *APISimpleSolver) Close() error {
	if s == nil || s.closed || s.last.closed {
		return nil
	}
	if !s.last.close() {
		return nil
	}
	return s.destroy()
}

func (s *APISimpleSolver) destroy() error {
	if s.closed {
		return nil
	}
	if s.explicitReady {
		if err := s.explicitContext.Del(&s.explicitParams); err != nil {
			return err
		}
		s.explicitReady = false
		s.explicitContext = nil
	}
	if err := s.dropSolver(); err != nil {
		return err
	}
	if s.release != nil {
		if err := s.release(); err != nil {
			return err
		}
		s.release = nil
	}
	s.owner, s.create = nil, nil
	s.closed = true
	return nil
}
