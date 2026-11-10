package opt

// Source: src/opt/opt_context.cpp context::update_solver and get_solver: a
// propositional MaxSAT problem (probe_bv: only Boolean, pseudo-Boolean and
// bit-vector symbols, a maxres-family engine, not pareto, no proofs) runs on
// mk_inc_sat_solver; m_solver then names that solver for every later
// check_sat, core and model.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	satsolver "github.com/glycerine/g3/pkg/sat/sat_solver"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// maxSMTBackend is the part of solver& the MaxSMT engines use.
type maxSMTBackend interface {
	Assert(ast.ASTHandle) error
	CheckSat([]ast.ASTHandle) (lbool.LBool, error)
	UnsatCore() ([]ast.ASTHandle, error)
	AcquireModel() (*model.ModelCore, error)
	FindMutexes([]ast.ASTHandle) (lbool.LBool, [][]ast.ASTHandle, error)
	NumAssertions() (uint32, error)
	Assertion(uint32) (ast.ASTHandle, error)
	Push() error
	Pop(uint32) error
	ReasonUnknown() (string, error)
	Labels() ([]usymbol.Symbol, error)
}

// solver is context::get_solver(): the incremental SAT solver once
// update_solver installed it, otherwise the opt_solver.
func (s *ArithmeticSolver) solver() maxSMTBackend {
	if s.sat != nil {
		return s.sat
	}
	return s.backend
}

// UpdateSolver is context::update_solver: when the problem is SAT-eligible
// it creates mk_inc_sat_solver(m, m_params) with minimize_core and
// minimize_core_partial set, and asserts the opt_solver's assertions.
func (s *ArithmeticSolver) UpdateSolver(params *uparams.ParamsRef) (result error) {
	if s == nil || s.backend == nil {
		return ast.ErrASTHandle
	}
	if !s.satCoreEnabled || s.sat != nil {
		return nil
	}
	c := s.config.Params
	p, err := c.NewValue()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, c.Del(&p)) }()
	if params != nil {
		if err = c.Copy(&p, params); err != nil {
			return err
		}
	}
	for _, name := range []string{"minimize_core_partial", "minimize_core"} {
		key := name
		if err = c.SetBoolName(&p, &key, true); err != nil {
			return err
		}
	}
	sat, err := satsolver.NewIncrementalSolver(s.manager, &p, s.config.ModelEvaluator)
	if err != nil {
		return err
	}
	n, err := s.backend.NumAssertions()
	if err != nil {
		return errors.Join(err, sat.Close())
	}
	for i := uint32(0); i < n; i++ {
		f, err := s.backend.Assertion(i)
		if err == nil {
			err = sat.Assert(f)
		}
		if err != nil {
			return errors.Join(err, sat.Close())
		}
	}
	s.sat = sat
	return nil
}
