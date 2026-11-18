package opt

// Source: src/opt/maxres.cpp maxres::operator(), mus_solver and found_optimum.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/util/lbool"
)

const maxResIterationLimit = 10000

func (e *maxResEngine) optimize(result *ArithmeticResult) (status lbool.LBool, optimal bool, err error) {
	if e == nil || e.solver == nil || e.solver.backend == nil || result == nil {
		return lbool.Undef, false, errMaxResState
	}
	defer func() {
		// mus_solver ends in found_optimum, which sets
		// m_found_feasible_optimum; primal_dual_solver sets it only when
		// a model has an empty correction set (processSearchSAT).
		if optimal && err == nil && e.strategy != maxResPrimalDual {
			e.foundFeasibleOptimum = true
		}
		if e.bestSet && e.index >= 0 && e.index < len(result.Values) {
			value := &result.Values[e.index]
			e.noteRat(result.Q.Copy(&value.SoftLower, &e.lower))
			e.noteRat(result.Q.Copy(&value.SoftUpper, &e.upper))
			value.SoftBoundsValid = true
		}
		if err == nil && e.ratErr != nil {
			status, optimal, err = lbool.Undef, false, e.ratErr
		}
	}()
	// maxsmt_solver_base::init reads its model captured before MaxRes mutex
	// discovery. Those preprocessing checks must not replace that candidate.
	if result.Model == nil {
		return lbool.Undef, false, fmt.Errorf("maxres source initial model missing: %w", errMaxResState)
	}
	// mus_solver: init() evaluates the softs in the model, then
	// init_local() finds the mutexes and adds the softs.
	if err = e.initEvaluate(result.Model); err != nil {
		return lbool.Undef, false, fmt.Errorf("maxres initial candidate: %w", err)
	}
	if err = e.initLocal(); err != nil {
		return lbool.Undef, false, err
	}
	if err = result.Model.IncRef(); err != nil {
		return lbool.Undef, false, err
	}
	e.initialCandidate = true
	err = e.updateCandidateModel(result, result.Model)
	e.initialCandidate = false
	if err != nil {
		return lbool.Undef, false, fmt.Errorf("maxres initial candidate: %w", err)
	}
	// init_local: is_sat = find_mutexes(new_soft); if (is_sat != l_true)
	// return is_sat;
	// mus_solver/primal_dual_solver: trace() right after init_local,
	// before its status is tested (AUDIT.md FUZZ-F5; g3trace only).
	if err = e.trace(); err != nil {
		return lbool.Undef, false, err
	}
	if e.mutexStopped {
		return e.mutexStatus, false, nil
	}
	for {
		if e.optimalByBounds() {
			if err = e.publishOptimum(result); err != nil {
				return lbool.Undef, false, err
			}
			if err = e.finishVerbose(true); err != nil {
				return lbool.Undef, false, err
			}
			return lbool.False, true, nil
		}
		if len(e.active) == 0 {
			if err = e.publishOptimum(result); err != nil {
				return lbool.Undef, false, err
			}
			if err = e.finishVerbose(false); err != nil {
				return lbool.Undef, false, err
			}
			return lbool.False, true, nil
		}
		var full bool
		status, full, err = e.checkHillClimb(result)
		if err != nil {
			return lbool.Undef, false, err
		}
		// if (m.canceled()) return l_undef;
		if canceled, e2 := e.trail.manager.Canceled(); e2 != nil || canceled {
			return lbool.Undef, false, e2
		}
		switch status {
		case lbool.True:
			done, e2 := e.processSearchSAT(result, full)
			if e2 != nil {
				return lbool.Undef, false, fmt.Errorf("maxres satisfiable round: %w", e2)
			}
			if done {
				// mus_solver's l_true branch: found_optimum(); return.
				// primal_dual_solver closed the gap and leaves its loop.
				if err = e.finishVerbose(false); err != nil {
					return lbool.Undef, false, err
				}
				return lbool.False, true, nil
			}
		case lbool.False:
			roundStatus, e2 := e.processSearchUNSAT(result)
			if e2 != nil {
				return lbool.Undef, false, e2
			}
			if roundStatus == lbool.False {
				// m_lower = m_upper ends the loop.
				if err = e.finishVerbose(true); err != nil {
					return lbool.Undef, false, err
				}
				return lbool.False, true, nil
			}
			if roundStatus == lbool.Undef {
				return lbool.Undef, false, nil
			}
		case lbool.Undef:
			return status, false, nil
		default:
			return lbool.Undef, false, errMaxResState
		}
	}
}
