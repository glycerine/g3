package qe

// Source: src/qe/qe_sat_tactic.cpp/.h, tactic lifecycle and goal application.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// QESATTactic supports model-based partial projection and selected scalar
// full projections, with optional strong context simplification.
type QESATTactic struct {
	self    *QESATTactic
	manager *ast.ASTManager
	config  Config
	params  uparams.ParamsRef
	options satTacticOptions
	// Lifetime members in source declaration order: m_solver, m_rewriter,
	// m_qe_rw and m_ctx_rewriter.
	aux    *smtcore.SMTSolver
	rw     *rewriter.TheoryRewriter
	qeRW   *simplifyRewriterStar
	strong *smtcore.ExprStrongContextSimplifier
	// Per-application state (m_vars, m_fml, m_Ms, m_solvers), released by
	// reset at the next application or at destruction.
	alt    *satAlternation
	search *satPartialSearch
	// assignments is m_assignments, which only destruction releases.
	assignments *satAssignments
	// Statistics snapshots: the source kernels' reset_statistics is empty,
	// so the port clears its own copies.
	levelStats          statistics.Statistics
	persistentStats     statistics.Statistics
	refs                uint32
	bytes               uint64
	paramsReady, closed bool
}

var _ tactic.Tactic = (*QESATTactic)(nil)

func NewQESATTactic(m *ast.ASTManager, params *uparams.ParamsRef, config Config) (*QESATTactic, error) {
	if m == nil || config.Context == nil || config.Globals == nil || config.Globals.Context != config.Context || config.Cooperation == nil {
		return nil, ast.ErrASTHandle
	}
	if err := config.Context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	size := uint64(reflect.TypeOf(QESATTactic{}).Size())
	if err := m.Budget().Reserve(size); err != nil {
		return nil, err
	}
	t := &QESATTactic{manager: m, config: config, options: newSATTacticOptions(), bytes: size, assignments: &satAssignments{manager: m}}
	t.self = t
	var err error
	t.params, err = config.Context.NewValue()
	t.paramsReady = err == nil
	if err == nil && params != nil {
		err = config.Context.Assign(&t.params, params)
		if err == nil {
			err = t.options.update(config.Context, &t.params)
		}
	}
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	if err = t.initMembers(); err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}

func (t *QESATTactic) solverConfig() smtcore.SMTSolverConfig {
	m, config := t.manager, t.config
	return smtcore.SMTSolverConfig{Params: config.Context, Globals: config.Globals, ModelEvaluator: model.ModelEvaluatorConfig{Context: config.Context, Globals: config.Globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: config.Cooperation, Trace: config.Trace}}
}

// initMembers constructs the sat_tactic members in source order.
func (t *QESATTactic) initMembers() (err error) {
	m, config := t.manager, t.config
	if t.aux, err = smtcore.NewSMTSolver(m, nil, "", t.solverConfig()); err != nil {
		return err
	}
	if t.rw, err = rewriter.NewTheoryRewriter(m, config.Context, config.Globals, nil); err != nil {
		return err
	}
	if t.qeRW, err = newSimplifyRewriterStar(m, config); err != nil {
		return err
	}
	t.strong, err = smtcore.NewExprStrongContextSimplifier(m, t.solverConfig())
	return err
}

// reset is sat_tactic::reset: the level solvers, then m_fml, m_Ms and m_vars.
func (t *QESATTactic) reset() error {
	if t.search != nil && t.search.levels != nil {
		if err := t.search.levels.closeSolvers(); err != nil {
			return err
		}
	}
	if t.alt != nil {
		if err := t.alt.releaseFormula(); err != nil {
			return err
		}
	}
	if t.search != nil {
		if err := t.search.Close(); err != nil {
			return err
		}
		t.search = nil
	}
	if t.alt != nil {
		if err := t.alt.Close(); err != nil {
			return err
		}
		t.alt = nil
	}
	return nil
}
func (t *QESATTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return t.config.Context.Check(&t.params)
}
func (t *QESATTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *QESATTactic) DecRef() error {
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
func (t *QESATTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	// ~sat_tactic: reset, then the members in reverse declaration order.
	if err := t.reset(); err != nil {
		return err
	}
	if t.strong != nil {
		if err := t.strong.Close(); err != nil {
			return err
		}
		t.strong = nil
	}
	if t.qeRW != nil {
		if err := t.qeRW.Close(); err != nil {
			return err
		}
		t.qeRW = nil
	}
	if t.rw != nil {
		if err := t.rw.Close(); err != nil {
			return err
		}
		t.rw = nil
	}
	if t.assignments != nil {
		if err := t.assignments.reset(); err != nil {
			return err
		}
	}
	if t.aux != nil {
		if err := t.aux.Close(); err != nil {
			return err
		}
		t.aux = nil
	}
	if t.paramsReady {
		if err := t.config.Context.Del(&t.params); err != nil {
			return err
		}
		t.paramsReady = false
	}
	t.manager.Budget().Release(t.bytes)
	t.bytes = 0
	t.closed = true
	return nil
}
func (t *QESATTactic) Cleanup() error { return t.open() }
func (t *QESATTactic) Reset() error   { return t.open() }
func (t *QESATTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewQESATTactic(m, nil, t.config)
}
func (t *QESATTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if p == nil {
		return ast.ErrASTHandle
	}
	if err := t.config.Context.Assign(&t.params, p); err != nil {
		return err
	}
	return t.options.update(t.config.Context, p)
}
func (t *QESATTactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	return collectSATTacticOptions(out)
}
func (t *QESATTactic) CollectStatistics(st *statistics.Statistics) {
	if t.open() != nil {
		return
	}
	st.Copy(t.levelStats)
	st.Copy(t.persistentStats)
}

// collectMemberStatistics is the m_solver and m_ctx_rewriter part of
// collect_statistics.
func (t *QESATTactic) collectMemberStatistics(st *statistics.Statistics) error {
	if err := t.aux.CollectStatistics(st); err != nil {
		return err
	}
	return t.strong.CollectStatistics(st)
}

func (t *QESATTactic) ResetStatistics() error {
	if err := t.open(); err != nil {
		return err
	}
	t.levelStats.Reset()
	t.persistentStats.Reset()
	return nil
}
func (t *QESATTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *QESATTactic) SetProgressCallback(func(uint32)) error { return t.open() }

func (t *QESATTactic) Apply(goal *tactic.Goal, out *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || goal.Manager() != t.manager || out == nil {
		return ast.ErrASTHandle
	}
	if err = tactic.FailIfProofGeneration("qe-sat", goal); err != nil {
		return err
	}
	if err = tactic.FailIfUnsatCoreGeneration("qe-sat", goal); err != nil {
		return err
	}
	m := t.manager
	canceled, err := m.Canceled()
	if err != nil {
		return err
	}
	if canceled {
		return tactic.NewTacticException(m.ResourceLimit().CancelMessage())
	}
	if err = t.config.Cooperation.Cooperate("qe-sat"); err != nil {
		return err
	}
	if err = t.reset(); err != nil {
		return err
	}
	t.levelStats.Reset()
	if err = out.Prepare(m); err != nil {
		return err
	}
	size, err := goal.Size()
	if err != nil {
		return err
	}
	forms := make([]ast.ASTHandle, size)
	for i := range forms {
		forms[i], err = goal.Form(uint32(i))
		if err != nil {
			return err
		}
	}
	conjunction, err := m.MkAndArgs(forms)
	if err != nil {
		return err
	}
	ref, err := m.NewRef(conjunction)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&ref)) }()
	preprocessed, err := t.qeRW.rewrite(conjunction)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&preprocessed)) }()
	conjunction, err = m.RefGet(preprocessed)
	if err != nil {
		return err
	}
	// m_fml = tmp releases the input conjunction.
	if err = m.SetRef(ref, conjunction); err != nil {
		return err
	}
	if t.alt, err = extractSATAlternation(m, t.config, conjunction); err != nil {
		return err
	}
	if t.search, err = newSATPartialSearch(m, t.config, t.alt, t.options, t.rw, t.aux, t.strong, t.assignments); err != nil {
		return err
	}
	if err = t.alt.releaseHoisted(); err != nil {
		return err
	}
	search := t.search
	defer func() {
		var level, persistent statistics.Statistics
		if collectErr := search.collectLevelStatistics(&level); collectErr != nil {
			err = errors.Join(err, collectErr)
			return
		}
		if collectErr := t.collectMemberStatistics(&persistent); collectErr != nil {
			err = errors.Join(err, collectErr)
			return
		}
		t.levelStats = level
		t.persistentStats = persistent
	}()
	truth, err := m.MkTrue()
	if err != nil {
		return err
	}
	result, err := search.qt(0, truth)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, result.Close()) }()
	h, err := m.RefGet(result.formula)
	if err != nil {
		return err
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	if m.IsFalse(h) {
		if err = goal.Assert(h); err != nil {
			return err
		}
	} else {
		if result.model == nil {
			return ast.ErrASTDomain
		}
		converter, e := tactic.NewStoredModel(result.model)
		if e != nil {
			return e
		}
		if e = out.SetModelConverter(converter); e != nil {
			return errors.Join(e, converter.Close())
		}
		if err = goal.Reset(); err != nil {
			return err
		}
	}
	return out.AddGoal(goal)
}
