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

// ProveUnsatByOrderAbstraction tries to refute the current assertions by
// relaxing quantified bit-vector order constraints to uninterpreted sorts.
// True establishes that the assertions are unsatisfiable. False is inconclusive:
// callers needing a decision or a concrete model should call Check. The
// independent proof attempt preserves this solver's last check, model and core.
func (s *APISimpleSolver) ProveUnsatByOrderAbstraction() (bool, error) {
	// Largest successful Raft obligation used 2.12 million resource steps.
	return s.proveUnsatByOrderAbstraction(3_000_000)
}

func (s *APISimpleSolver) proveUnsatByOrderAbstraction(maxSteps uint32) (bool, error) {
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
	script, applicable, err := order.Relax(parent, assertions)
	if err != nil || !applicable || limit.CancelFlag() {
		return false, err
	}
	return checkOrderAbstraction(parent, script, maxSteps)
}

func checkOrderAbstraction(parent *ast.ASTManager, script string, maxSteps uint32) (proved bool, err error) {
	command, err := cmd_context.NewAuxiliaryContextWithBudget(parent.Budget())
	if err != nil {
		return false, err
	}
	command.UseAPISolverInitialization()
	ctx := &SMTContext{smtContextState: &smtContextState{command: command}}
	defer func() {
		err = errors.Join(err, ctx.Close())
		if err != nil {
			proved = false
		}
	}()
	m, err := ctx.Manager()
	if err != nil {
		return false, err
	}
	limit, parentLimit := m.ResourceLimit(), parent.ResourceLimit()
	limit.RunSignals = parentLimit.RunSignals
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
	child, err := ctx.NewSolver()
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, child.Close()) }()
	params, err := ctx.NewParameterSet()
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, params.Close()) }()
	if err = params.SetBool("smt.macro_finder", true); err != nil {
		return false, err
	}
	if err = child.SetParameters(params); err != nil {
		return false, err
	}
	if err = child.LoadSMT2String(script); err != nil {
		return false, err
	}
	answer, err := child.Check()
	if !limit.IncOffset(0) || parentLimit.CancelFlag() {
		return false, nil
	}
	return answer == lbool.False && err == nil, err
}
