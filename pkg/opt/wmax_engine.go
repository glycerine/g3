package opt

// Source: src/opt/wmax.cpp weighted-theory MaxSAT search.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/numeral"
)

type wmaxEngine struct {
	solver       *ArithmeticSolver
	objective    *arithmeticObjective
	index        int
	owner        maxSMTASTOwner
	prepared     arithmeticObjective
	theory       *smtcore.SMTWeightedMaxTheory
	lower, upper numeral.Rational
	checks       uint32
	mutexes      uint32
	mutexTerms   uint32
	closed       bool
	mutexStatus  lbool.LBool
	mutexStopped bool
}

func newWMaxEngine(s *ArithmeticSolver, objective *arithmeticObjective, index int) (result *wmaxEngine, resultErr error) {
	if s == nil || objective == nil {
		return nil, errMaxResState
	}
	prepared := &maxResEngine{
		solver: s, objective: objective, index: index,
		weights: make(map[ast.ASTHandle]*numeral.Rational), proxies: make(map[ast.ASTHandle]ast.ASTHandle),
		lower: numeral.NewRational(), upper: numeral.NewRational(),
	}
	prepared.baseTrail.manager = s.manager
	o := newRatOps(s.manager)
	o.set(&prepared.lower, &objective.softOffset)
	terms, status, err := prepared.preprocessMutexes(objective)
	if err = o.joined(err); err != nil {
		return nil, errors.Join(err, prepared.baseTrail.close(), prepared.releaseWeights())
	}
	defer func() {
		resultErr = errors.Join(resultErr, releaseWeightedTerms(prepared.q(), terms), prepared.releaseWeights())
	}()
	e := &wmaxEngine{solver: s, objective: objective, index: index, owner: prepared.baseTrail, lower: numeral.NewRational(), upper: numeral.NewRational()}
	if status != lbool.True {
		e.mutexStatus, e.mutexStopped = status, true
		// wmax::operator() returns find_mutexes' status before it builds
		// the wmaxsmt theory.
		prepared.baseTrail = maxSMTASTOwner{}
		e.owner.manager = s.manager
		return e, nil
	}
	prepared.baseTrail = maxSMTASTOwner{}
	o.set(&e.lower, &prepared.lower)
	e.mutexes, e.mutexTerms = prepared.stats.Mutexes, prepared.stats.MutexTerms
	e.owner.manager = s.manager
	e.prepared.soft = true
	e.prepared.softOffset = o.zero()
	e.prepared.softOwned = true
	for _, term := range terms {
		ref, cause := s.manager.NewRef(term.expression)
		if cause != nil {
			return nil, errors.Join(cause, e.close())
		}
		e.owner.refs = append(e.owner.refs, ref)
		e.prepared.softTerms = append(e.prepared.softTerms, ref)
		e.prepared.softWeights = append(e.prepared.softWeights, o.clone(&term.weight))
	}
	if !o.ok() {
		return nil, errors.Join(o.err, e.close())
	}
	e.theory, err = s.backend.EnsureWeightedMaxTheory(s.filter)
	if err != nil {
		return nil, errors.Join(err, e.close())
	}
	for i := range e.prepared.softTerms {
		formula, cause := s.manager.RefGet(e.prepared.softTerms[i])
		if cause != nil {
			return nil, errors.Join(cause, e.close())
		}
		slack, assertion, cause := e.theory.AddWeighted(formula, &e.prepared.softWeights[i])
		if cause == nil {
			slack, cause = e.owner.hold(slack, nil)
		}
		if cause == nil {
			assertion, cause = e.owner.hold(assertion, nil)
		}
		if cause == nil {
			cause = s.solver().Assert(assertion)
		}
		if cause != nil {
			return nil, errors.Join(cause, e.close())
		}
	}
	return e, nil
}

func (e *wmaxEngine) close() error {
	if e == nil || e.closed {
		return nil
	}
	e.closed = true
	e.prepared.softTerms = nil
	o := newRatOps(e.solver.manager)
	o.del(&e.upper)
	o.del(&e.lower)
	err := errors.Join(e.prepared.releaseSoftRationals(e.solver.manager), o.err, e.owner.close())
	if e.theory != nil {
		err = errors.Join(err, e.theory.ResetLocal())
		e.theory = nil
	}
	return err
}

// scorePrepared returns caller-owned satisfied and penalty sums.
func (e *wmaxEngine) scorePrepared(mdModel *model.ModelCore) (satisfied, penalty numeral.Rational, resultErr error) {
	o := newRatOps(e.solver.manager)
	satisfied, penalty = o.zero(), o.zero()
	expressions := make([]ast.ASTHandle, len(e.prepared.softTerms))
	for i := range e.prepared.softTerms {
		value, err := e.solver.manager.RefGet(e.prepared.softTerms[i])
		if err != nil {
			return satisfied, penalty, err
		}
		expressions[i] = value
	}
	assignment, err := wmaxModelAssignment(e.solver, mdModel, expressions)
	if err != nil {
		return satisfied, penalty, err
	}
	for i, yes := range assignment {
		if yes {
			o.add(&satisfied, &satisfied, &e.prepared.softWeights[i])
		} else {
			o.add(&penalty, &penalty, &e.prepared.softWeights[i])
		}
	}
	return satisfied, penalty, o.err
}

func (e *wmaxEngine) publishAssignment(result *ArithmeticResult) error {
	expressions := make([]ast.ASTHandle, len(e.objective.softTerms))
	for i := range e.objective.softTerms {
		value, err := e.solver.manager.RefGet(e.objective.softTerms[i])
		if err != nil {
			return err
		}
		expressions[i] = value
	}
	assignment, err := wmaxModelAssignment(e.solver, result.Model, expressions)
	if err != nil {
		return err
	}
	o := newRatOps(e.solver.manager)
	cost, satisfied := o.clone(&e.objective.softOffset), o.zero()
	defer o.del(&satisfied)
	defer o.del(&cost)
	for i, yes := range assignment {
		if yes {
			o.add(&satisfied, &satisfied, &e.objective.softWeights[i])
		} else {
			o.add(&cost, &cost, &e.objective.softWeights[i])
		}
	}
	if !o.ok() {
		return o.err
	}
	if err = setSoftObjectiveValue(result, e.index, &cost, &satisfied, assignment); err != nil {
		return err
	}
	value := &result.Values[e.index]
	o.set(&value.SoftLower, &e.lower)
	o.set(&value.SoftUpper, &e.upper)
	value.SoftBoundsValid = true
	return o.err
}

func (e *wmaxEngine) optimize(result *ArithmeticResult) (status lbool.LBool, optimal bool, resultErr error) {
	if e != nil && e.mutexStopped {
		return e.mutexStatus, false, nil
	}
	if e == nil || e.solver == nil || e.theory == nil || result == nil || result.Model == nil {
		return lbool.Undef, false, errMaxResState
	}
	o := newRatOps(e.solver.manager)
	defer func() { resultErr = o.joined(resultErr) }()
	satisfied, penalty, err := e.scorePrepared(result.Model)
	o.del(&satisfied)
	if err != nil {
		o.del(&penalty)
		return lbool.Undef, false, err
	}
	o.add(&e.upper, &e.lower, &penalty)
	err = e.theory.InitMinCost(&penalty)
	o.del(&penalty)
	if err != nil || !o.ok() {
		return lbool.Undef, false, err
	}
	// rational limit (m_upper - m_lower)
	limit := o.zero()
	defer o.del(&limit)
	// trace_bounds("wmax") (AUDIT.md FUZZ-F5; g3trace builds only).
	if diagnostic.VerboseProgress {
		if err = e.solver.traceBounds("wmax", e.index, &e.lower, &e.upper); err != nil {
			return lbool.Undef, false, err
		}
	}
	status = lbool.True
	m := e.solver.manager
	// while (!m.canceled() && m_lower < m_upper)
	for {
		canceled, cause := m.Canceled()
		if cause != nil {
			return lbool.Undef, false, cause
		}
		if canceled || o.cmp(&e.lower, &e.upper) >= 0 || !o.ok() {
			break
		}
		status, err = e.solver.solver().CheckSat(nil)
		if err != nil {
			return lbool.Undef, false, err
		}
		e.checks++
		// if (m.canceled()) is_sat = l_undef;
		if canceled, cause = m.Canceled(); cause != nil {
			return lbool.Undef, false, cause
		} else if canceled {
			status = lbool.Undef
		}
		if status != lbool.True {
			break
		}
		isOptimal, cause := e.theory.IsOptimal()
		if cause != nil {
			return lbool.Undef, false, cause
		}
		if isOptimal {
			// m_upper = m_lower + wth().get_cost()
			if cause = e.theory.Cost(&limit); cause != nil {
				return lbool.Undef, false, cause
			}
			o.add(&e.upper, &e.lower, &limit)
			if cause = e.solver.replaceModel(result); cause != nil {
				return lbool.Undef, false, cause
			}
		}
		block, cause := e.theory.MakeBlock()
		if cause == nil {
			block, cause = e.owner.hold(block, nil)
		}
		if cause == nil {
			cause = e.solver.solver().Assert(block)
		}
		if cause != nil {
			return lbool.Undef, false, cause
		}
		o.sub(&limit, &e.upper, &e.lower)
		if !o.ok() {
			return lbool.Undef, false, nil
		}
		if err = e.theory.InitMinCost(&limit); err != nil {
			return lbool.Undef, false, err
		}
		if diagnostic.VerboseProgress {
			if err = e.solver.traceBounds("wmax", e.index, &e.lower, &e.upper); err != nil {
				return lbool.Undef, false, err
			}
		}
	}
	// if (!m.canceled() && is_sat == l_undef && m_lower == m_upper)
	canceled, err := m.Canceled()
	if err != nil {
		return lbool.Undef, false, err
	}
	if !canceled && status == lbool.Undef && o.cmp(&e.lower, &e.upper) == 0 {
		status = lbool.True
	}
	if status == lbool.False {
		o.set(&e.lower, &e.upper)
		status = lbool.True
	}
	if !o.ok() {
		return lbool.Undef, false, nil
	}
	if err = e.publishAssignment(result); err != nil {
		return lbool.Undef, false, err
	}
	return status, status == lbool.True && o.cmp(&e.lower, &e.upper) == 0, nil
}

func (e *wmaxEngine) statistics() MaxSMTStatistics {
	return MaxSMTStatistics{Engine: "wmax", Checks: e.checks, Mutexes: e.mutexes, MutexTerms: e.mutexTerms}
}
