package opt

// Source: src/opt/maxres.cpp maxres::operator(), mus_solver and found_optimum.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *maxResEngine) optimize(result *ArithmeticResult) (status lbool.LBool, optimal bool, err error) {
	if e == nil || e.solver == nil || e.solver.backend == nil || result == nil {
		return lbool.Undef, false, errMaxResState
	}
	// Mutex discovery uses its own backend query. Re-establish the source
	// solver's candidate model before deriving the first upper bound.
	e.stats.Checks++
	status, err = e.solver.backend.CheckSat(nil)
	if err != nil || status != lbool.True {
		return status, false, err
	}
	// The enclosing maxsmt driver has already retained the first satisfiable
	// model. Native MaxRes immediately derives its upper bound and assignment.
	if err = e.updateCandidate(result); err != nil {
		return lbool.Undef, false, fmt.Errorf("maxres initial candidate: %w", err)
	}
	e.sortAssumptions()
	for {
		if e.optimalByBounds() {
			if err = e.publishOptimum(result); err != nil {
				return lbool.Undef, false, err
			}
			return lbool.False, true, nil
		}
		if len(e.active) == 0 {
			if err = e.publishOptimum(result); err != nil {
				return lbool.Undef, false, err
			}
			return lbool.False, true, nil
		}
		var full bool
		status, full, err = e.checkHillClimb(result)
		if err != nil {
			return lbool.Undef, false, err
		}
		switch status {
		case lbool.True:
			done, e2 := e.processSearchSAT(result, full)
			if e2 != nil {
				return lbool.Undef, false, fmt.Errorf("maxres satisfiable round: %w", e2)
			}
			if done {
				return lbool.False, true, nil
			}
		case lbool.False:
			if err = e.processSearchUNSAT(result); err != nil {
				return lbool.Undef, false, err
			}
		case lbool.Undef:
			return status, false, nil
		default:
			return lbool.Undef, false, errMaxResState
		}
	}
}
