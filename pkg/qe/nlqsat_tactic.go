package qe

// Source: src/qe/nlqsat.cpp:628-713,817-908.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/nlsat"
	nlsattactic "github.com/glycerine/g3/pkg/nlsat/tactic"
	"github.com/glycerine/g3/pkg/tactic"
	tacticcore "github.com/glycerine/g3/pkg/tactic/core"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	primegen "github.com/glycerine/g3/pkg/util/prime"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type NLQSATTactic struct {
	self       *NLQSATTactic
	manager    *ast.ASTManager
	config     Config
	statistics map[string]float64
	bytes      uint64
	refs       uint32
	running    bool
	closed     bool
}

var _ tactic.Tactic = (*NLQSATTactic)(nil)

func NewNLQSATTactic(manager *ast.ASTManager, config Config) (*NLQSATTactic, error) {
	if manager == nil || config.Context == nil || config.Globals == nil || config.Globals.Context != config.Context || config.Cooperation == nil {
		return nil, arena.ErrConfig
	}
	bytes := uint64(reflect.TypeOf(NLQSATTactic{}).Size())
	if err := manager.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	t := &NLQSATTactic{manager: manager, config: config, bytes: bytes}
	t.self = t
	return t, nil
}

func (t *NLQSATTactic) open() error {
	if t == nil || t.self != t || t.manager == nil {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	if t.running {
		return ast.ErrASTPending
	}
	return t.config.Context.CheckServices(t.manager.Symbols, t.manager.NumeralService())
}
func (t *NLQSATTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *NLQSATTactic) DecRef() error {
	if err := t.open(); err != nil {
		return err
	}
	if t.refs == 0 {
		return arena.ErrBounds
	}
	t.refs--
	if t.refs == 0 {
		return t.Close()
	}
	return nil
}
func (t *NLQSATTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if t.running {
		return ast.ErrASTPending
	}
	t.manager.Budget().Release(t.bytes)
	t.bytes, t.closed = 0, true
	return nil
}

func (t *NLQSATTactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || result == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = tactic.FailIfProofGeneration("nlqsat", goal); err != nil {
		return err
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	t.running = true
	defer func() { t.running = false }()
	forms, err := goal.AppendFormulas(nil)
	if err != nil {
		return err
	}
	formula, err := t.manager.MkAndArray(forms)
	if err != nil {
		return err
	}
	formulaRef, err := t.manager.NewRef(formula)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, t.manager.DelRef(&formulaRef)) }()
	preparation, err := newQSATPreparation(t.manager, t.config)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, preparation.Close()) }()
	if err = preparation.hoist(qsatSAT, formulaRef); err != nil {
		return err
	}
	formula, err = t.manager.RefGet(formulaRef)
	if err != nil {
		return err
	}
	boolean, err := t.manager.MkBoolSort()
	if err != nil {
		return err
	}
	truth, err := t.manager.MkFreshConst("nlqsat-is-true", boolean)
	if err != nil {
		return err
	}
	definition, err := t.manager.MkIff(truth, formula)
	if err != nil {
		return err
	}
	cnfGoal, err := tactic.NewGoal(t.manager, false, false, false)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, cnfGoal.Close()) }()
	if err = cnfGoal.Assert(definition); err != nil {
		return err
	}
	services := model.ModelEvaluatorConfig{Context: t.config.Context, Globals: t.config.Globals, Memory: t.manager.NumeralService().RuntimeOwner(), Cooperation: t.config.Cooperation, Trace: t.config.Trace}
	cnf, err := tacticcore.NewTseitinCNFCoreTactic(t.manager, t.config.Context, t.config.Globals, nil, services)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, cnf.Close()) }()
	cnfResult := &tactic.TacticResult{}
	defer func() { err = errors.Join(err, cnfResult.Close()) }()
	if err = cnf.Apply(cnfGoal, cnfResult); err != nil {
		return err
	}

	algebraic, err := t.manager.ArithmeticAlgebraicManager()
	if err != nil {
		return err
	}
	rationals, err := algebraic.RationalManager()
	if err != nil {
		return err
	}
	primes, err := primegen.NewGenerator(t.manager.Budget())
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, primes.Close()) }()
	polynomials, err := polynomial.NewMultivariatePolynomialManager(polynomial.MultivariatePolynomialManagerConfig{
		Core:          polynomial.MultivariatePolynomialConfig{Integer: rationals.Integers(), Rationals: rationals, Checkpoint: t.manager.Limit.Inc, Cooperation: func() error { return t.config.Cooperation.Cooperate("nlqsat") }},
		Factorization: polynomial.UPolynomialFactorizationConfig{Primes: primes}, Trace: t.config.Trace,
	})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, polynomials.CloseChecked()) }()
	solver, err := nlsat.NewSolver(nlsat.SolverConfig{Algebraic: algebraic, Polynomial: polynomials, Limit: &t.manager.Limit, SimplifyCore: true, Factor: false})
	if err != nil {
		return err
	}
	defer func() {
		if stats, cause := solver.Statistics(); cause == nil {
			t.statistics = stats
		} else {
			err = errors.Join(err, cause)
		}
		err = errors.Join(err, solver.Close())
	}()
	compiler, err := nlsattactic.NewGoalCompiler(t.manager, solver)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, compiler.Close()) }()
	if err = compiler.SetFactor(false); err != nil {
		return err
	}
	if err = compiler.Compile(cnfGoal); err != nil {
		return tactic.NewTacticException(err.Error())
	}
	search, err := newNLQSearchFromCompiler(solver, compiler, preparation.variables, truth)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, search.close()) }()
	status, err := search.check()
	if err != nil {
		return tactic.NewTacticException(err.Error())
	}
	if status == lbool.Undef {
		return tactic.NewTacticException("search failed")
	}
	if err = goal.Reset(); err != nil {
		return err
	}
	if status == lbool.False {
		falsity, cause := t.manager.MkFalse()
		if cause != nil {
			return cause
		}
		if err = goal.Assert(falsity); err != nil {
			return err
		}
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	return result.AddGoal(goal)
}

func (t *NLQSATTactic) UpdateParams(*uparams.ParamsRef) error         { return t.open() }
func (t *NLQSATTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *NLQSATTactic) CollectStatistics() map[string]float64 {
	out := make(map[string]float64, len(t.statistics))
	for k, v := range t.statistics {
		out[k] = v
	}
	return out
}
func (t *NLQSATTactic) ResetStatistics() error {
	if err := t.open(); err != nil {
		return err
	}
	t.statistics = nil
	return nil
}
func (t *NLQSATTactic) Cleanup() error                         { return t.open() }
func (t *NLQSATTactic) Reset() error                           { return t.Cleanup() }
func (t *NLQSATTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *NLQSATTactic) SetProgressCallback(func(uint32)) error { return t.open() }
func (t *NLQSATTactic) Translate(manager *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewNLQSATTactic(manager, t.config)
}
