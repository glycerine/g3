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
// relaxation to uninterpreted ordered sorts.
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
	bits *orderBooleanSolver
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
func (b *OrderProofBatch) Prove() (bool, error) {
	if b == nil || b.state == nil || b.state.source == nil {
		return false, ErrSMTContextOwner
	}
	return b.state.prove(orderProofSteps, true)
}

// Close releases the batch's private kernel. The source remains usable.
func (b *OrderProofBatch) Close() error {
	if b == nil || b.state == nil {
		return nil
	}
	b.state.source = nil
	return b.state.release()
}

func (p *orderProofState) prove(maxSteps uint32, reuse bool) (proved bool, err error) {
	defer func() {
		if !reuse || !proved {
			if closeErr := p.release(); closeErr != nil {
				// A failed cleanup can leave assertions in the auxiliary solver.
				// Only Close may retry cleanup after this point.
				p.source = nil
				proved, err = false, errors.Join(err, closeErr)
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
	proof, applicable, err := order.ForProof(parent, assertions, maxSteps)
	if err != nil || !applicable || limit.CancelFlag() {
		return false, err
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
