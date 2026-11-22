package api

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/api/internal/order"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// The largest successful Raft obligation used 2.12 million resource steps.
const orderProofSteps = 3_000_000

// ProveUnsatByOrderAbstraction tries to refute the current assertions by
// checking for a contradictory ground instance of a quantified bit-vector
// assertion. Remaining problems can use exact two-element domains or a
// relaxation to uninterpreted ordered sorts. For quantified problems over
// uninterpreted sorts, it can refute ground instances at the final goal's constants.
// A quantified goal can also be refuted by its Boolean structure and the
// one-way implications connecting quantified formulas to their ground instances.
// Ground Boolean goals may use the ground conjuncts of their premises. A
// violated conjunctive postcondition can also use ground relation instances.
// For a constant true goal, ground facts and short universal premises may
// already contradict each other.
// For goals with several quantifiers over the same domain, a private ground
// solver also checks instances at fresh witnesses and observed applications.
// True establishes that the assertions are unsatisfiable. False is inconclusive:
// callers needing a decision or a concrete model should call Check. The
// independent proof attempt preserves this solver's last check, model and core.
func (s *APISimpleSolver) ProveUnsatByOrderAbstraction() (bool, error) {
	return s.proveUnsatByOrderAbstraction(orderProofSteps)
}

func (s *APISimpleSolver) proveUnsatByOrderAbstraction(maxSteps uint32) (bool, error) {
	proof := orderProofState{source: s}
	return proof.prove(maxSteps, false)
}

// OrderProofBatch reuses a private solver across successive refutations
// on one source solver. Each attempt reads the source's current assertions.
// Close the batch when the sequence of related checks finishes.
type OrderProofBatch struct{ state *orderProofState }

type orderProofState struct {
	witnessPremises    *witnessPremises
	definitions        *premiseDefinitions
	previousGoalProved bool
	recent             *recentRefutations
	bits               *orderBooleanSolver
	// Keep a failed side-condition cleanup reachable for release's retry.
	pendingCondition *orderProofState
	source           *APISimpleSolver
	ctx              *SMTContext
	child            *APISimpleSolver
	parser           *cmd_context.Context
	declared         map[string]string
	premises         string
}

// BeginOrderProofBatch creates a batch confined to the source solver's owner.
// A proof that needs search opens the auxiliary solver on demand.
func (s *APISimpleSolver) BeginOrderProofBatch() *OrderProofBatch {
	return &OrderProofBatch{state: &orderProofState{source: s}}
}

// Prove has the same result contract as ProveUnsatByOrderAbstraction.
// A successful attempt removes or disables its query suffix. Later attempts
// can reuse identical premises in the same generated language. An inconclusive
// attempt releases the private kernel before the caller's concrete check.
// The batch can also reuse a recent concrete UNSAT result under exactly
// the same ordered assertions. Each source solver observes its own checks.
func (b *OrderProofBatch) Prove() (bool, error) {
	if b == nil || b.state == nil || b.state.source == nil {
		return false, ErrSMTContextOwner
	}
	b.state.source.pendingRefutation = nil
	return b.state.prove(orderProofSteps, true)
}

// ProveWithGoal tries to refute the current assertions together with one
// temporary Boolean goal before a fresh solver's first push. It preserves
// the source assertions, scope level and result. False is inconclusive.
// A caller can then push the goal and check it through the ordinary solver.
func (b *OrderProofBatch) ProveWithGoal(goal ast.ASTHandle) (proved bool, err error) {
	if b == nil || b.state == nil || b.state.source == nil {
		return false, ErrSMTContextOwner
	}
	p := b.state
	defer func() { p.previousGoalProved = proved && err == nil }()
	s := p.source
	if err = s.open(); err != nil {
		return false, err
	}
	m, err := s.owner.command.Manager()
	if err != nil {
		return false, err
	}
	boolean, err := m.HasBoolSort(goal)
	if err != nil {
		return false, err
	}
	if !boolean {
		return false, ast.ErrASTDomain
	}
	combined, ok := s.solver.(*solver.CombinedSolver)
	if !ok || !combined.CanDeferFirstPush() {
		return false, nil
	}
	if s.pendingRefutation == p.recent {
		s.pendingRefutation = nil
	}
	held, err := m.NewRef(goal)
	if err != nil {
		return false, err
	}
	defer func() {
		if e := m.DelRef(&held); e != nil {
			proved, err = false, errors.Join(err, e)
		}
	}()
	return p.proveWithGoal(orderProofSteps, true, goal)
}

// Close releases the batch's private kernel. The source remains usable.
func (b *OrderProofBatch) Close() error {
	if b == nil || b.state == nil {
		return nil
	}
	if s := b.state.source; s != nil && s.pendingRefutation == b.state.recent {
		s.pendingRefutation = nil
	}
	b.state.witnessPremises = nil
	b.state.definitions = nil
	b.state.recent = nil
	b.state.source = nil
	return b.state.release()
}

func (p *orderProofState) prove(maxSteps uint32, reuse bool) (bool, error) {
	return p.proveWithGoal(maxSteps, reuse, ast.ASTHandle{})
}

func (p *orderProofState) proveWithGoal(maxSteps uint32, reuse bool, goal ast.ASTHandle) (proved bool, err error) {
	var candidate *recentRefutations
	defer func() {
		if !reuse || !proved {
			if closeErr := p.release(); closeErr != nil {
				// A failed cleanup can leave assertions in the auxiliary solver.
				// Only Close may retry cleanup after this point.
				p.source = nil
				proved, err = false, errors.Join(err, closeErr)
			}
		}
		if candidate != nil && err == nil {
			if proved {
				candidate.remember()
			} else if goal.IsNull() {
				p.source.pendingRefutation = candidate
			}
		}
	}()
	s := p.source
	if err := s.use(); err != nil {
		return false, err
	}
	parent, err := s.solver.Manager()
	if err != nil {
		return false, err
	}
	limit := parent.ResourceLimit()
	if maxSteps == 0 || limit.CancelFlag() || limit.Limit != 0 || parent.Budget().InjectionArmed() {
		return false, nil
	}
	tracked, err := s.solver.NumAssumptions()
	if err != nil || tracked != 0 {
		return false, err
	}
	timeout, steps, err := s.owner.command.ExecutionLimits()
	if err != nil {
		return false, err
	}
	if s.explicitReady {
		timeoutKey, limitKey := "timeout", "rlimit"
		timeout, err = s.explicitContext.GetUIntName(&s.explicitParams, &timeoutKey, timeout)
		if err != nil {
			return false, err
		}
		steps, err = s.explicitContext.GetUIntName(&s.explicitParams, &limitKey, steps)
		if err != nil {
			return false, err
		}
	}
	if steps != 0 || timeout != 0 && timeout != math.MaxUint32 {
		return false, nil
	}
	assertions, err := solver.Assertions(s.solver)
	if err != nil {
		return false, err
	}
	if !goal.IsNull() {
		assertions = append(assertions, goal)
	}
	if reuse && maxSteps == orderProofSteps && len(assertions) > 0 && len(assertions)-1 <= maxRecentPremises && hasOriginalAssertions(s.solver) {
		if p.recent == nil {
			p.recent = new(recentRefutations)
		}
		if p.recent.lookup(assertions) {
			return !limit.CancelFlag(), nil
		}
		candidate = p.recent
	}
	proved, err = p.proveAssertions(parent, assertions, maxSteps, reuse)
	if err != nil || proved || goal.IsNull() || !p.previousGoalProved || limit.CancelFlag() {
		return proved, err
	}
	return p.proveRelevantPremises(parent, assertions)
}

func (p *orderProofState) proveAssertions(parent *ast.ASTManager, assertions []ast.ASTHandle, maxSteps uint32, reuse bool) (bool, error) {
	s := p.source
	limit := parent.ResourceLimit()
	proof, applicable, err := order.ForProof(parent, assertions, maxSteps)
	if err != nil || limit.CancelFlag() {
		return false, err
	}
	if !applicable {
		bits, err := order.ProjectQuantifiedGoal(parent, assertions, maxSteps)
		if err != nil || limit.CancelFlag() {
			return false, err
		}
		searchSteps := maxSteps
		if bits == nil {
			// The incremental solver can answer immediately when preprocessing
			// has already produced false. Avoid projecting its ground premises.
			incremental, ok := s.solver.(interface{ HasInconsistentAssertions() bool })
			consistent := !ok || !incremental.HasInconsistentAssertions()
			if consistent {
				bits, err = order.ProjectGroundBooleans(parent, assertions, maxSteps)
				if err != nil || limit.CancelFlag() {
					return false, err
				}
			}
			if bits == nil {
				if consistent && hasOriginalAssertions(s.solver) {
					subset, err := p.shortPremises(parent, assertions)
					if err != nil || limit.CancelFlag() {
						return false, err
					}
					if subset != nil {
						// The index admits at most 16,000 distinct premises.
						// Bound speculative work in proportion to the selected input.
						steps := min(maxSteps, 80_000, 192*uint32(len(subset.selected)))
						proved, err := p.checkProofPlan(parent, subset, steps)
						if err != nil || proved || limit.CancelFlag() {
							return proved, err
						}
					}
				}
				return p.proveGoalInstances(parent, assertions, maxSteps)
			}
			searchSteps = min(maxSteps, 80_000)
		}
		proof.Boolean, proof.SearchSteps = bits, searchSteps
	}
	if proof.Refuted {
		return true, nil
	}
	if proof.Boolean != nil {
		if p.ctx != nil {
			if err := p.release(); err != nil {
				return false, err
			}
		}
		return p.checkBoolean(parent, proof.Boolean, proof.SearchSteps)
	}
	if p.bits != nil {
		if err := p.release(); err != nil {
			return false, err
		}
	}
	if reuse && p.needsNewDeclarations(proof.Order) {
		if err := p.release(); err != nil {
			return false, err
		}
	}
	if p.ctx == nil {
		if err := p.start(parent); err != nil {
			return false, err
		}
	}
	if c := proof.Conditional; c != nil && proof.SearchSteps >= 8 {
		allowance := min(proof.SearchSteps/8, 150_000)
		proof.SearchSteps -= allowance
		side := orderProofState{source: s}
		err := side.start(parent)
		var established bool
		if err == nil {
			established, err = side.check(parent, c.Condition, allowance, false)
		}
		if closeErr := side.release(); closeErr != nil {
			p.pendingCondition = &side
			return false, errors.Join(err, closeErr)
		}
		if err != nil || limit.CancelFlag() {
			return false, err
		}
		if established {
			proof.Order = c.Primary
		}
	}
	return p.check(parent, proof.Order, proof.SearchSteps, reuse)
}

func (p *orderProofState) start(parent *ast.ASTManager) (err error) {
	command, err := cmd_context.NewRefutationContext(parent.Budget())
	if err != nil {
		return err
	}
	command.UseAPISolverInitialization()
	p.ctx = &SMTContext{smtContextState: &smtContextState{command: command}}
	if _, err = p.ctx.Manager(); err != nil {
		return err
	}
	if p.child, err = p.ctx.NewSimpleSolver(); err != nil {
		return err
	}
	params, err := p.ctx.NewParameterSet()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, params.Close()) }()
	if err := params.SetBool("smt.macro_finder", true); err != nil {
		return err
	}
	return p.child.SetParameters(params)
}

func (p *orderProofState) release() error {
	if p.pendingCondition != nil {
		if err := p.pendingCondition.release(); err != nil {
			return err
		}
		p.pendingCondition = nil
	}
	if p.bits != nil {
		if err := p.bits.close(); err != nil {
			return err
		}
		p.bits = nil
	}
	if err := p.closeParser(); err != nil {
		return err
	}
	if p.child != nil {
		if err := p.child.Close(); err != nil {
			return err
		}
		p.child = nil
	}
	if p.ctx != nil {
		if err := p.ctx.Close(); err != nil {
			return err
		}
		p.ctx = nil
	}
	return nil
}

func (p *orderProofState) check(parent *ast.ASTManager, problem order.Problem, maxSteps uint32, reuse bool) (bool, error) {
	m, err := p.ctx.Manager()
	if err != nil {
		return false, err
	}
	limit, parentLimit := m.ResourceLimit(), parent.ResourceLimit()
	limit.RunSignals = parentLimit.RunSignals
	if p.parser != nil {
		p.parser.SetRunSignals(parentLimit.RunSignals)
	}
	defer func() {
		if p.parser != nil {
			p.parser.SetRunSignals(nil)
		}
		limit.RunSignals = nil
	}()
	if err = limit.Push(maxSteps); err != nil {
		return false, err
	}
	defer limit.Pop()
	if err = parentLimit.PushChild(limit); err != nil {
		return false, err
	}
	defer parentLimit.PopChild()
	if parentLimit.CancelFlag() {
		return false, nil
	}
	// Assertion preprocessing in the simple solver also polls the limit.
	if reuse {
		err = p.loadBatch(problem)
	} else {
		err = p.child.LoadSMT2String(problem.Text)
	}
	var answer lbool.LBool
	if err == nil {
		answer, err = p.child.Check()
	}
	if !limit.IncOffset(0) || parentLimit.CancelFlag() {
		return false, nil
	}
	if err != nil || answer != lbool.False {
		return false, err
	}
	if reuse {
		if err := p.child.Pop(1); err != nil {
			return false, err
		}
	}
	return true, nil
}
