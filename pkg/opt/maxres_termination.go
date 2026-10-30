package opt

// Source: src/opt/maxres.cpp found_optimum and lower/upper loop guard.

import "github.com/glycerine/g3/pkg/util/lbool"

func (e *maxResEngine) optimalByBounds() bool {
	return e.bestSet && e.lower.Cmp(&e.upper) >= 0
}

func (e *maxResEngine) publishOptimum(result *ArithmeticResult) error {
	// Core/correction transformations can establish the optimum on an
	// unsatisfiable assumption check. Recheck the retained hard formulas to
	// publish their last valid source model and assignment.
	e.stats.Checks++
	status, err := e.solver.backend.CheckSat(nil)
	if err != nil {
		return err
	}
	if status != lbool.True {
		return errMaxResState
	}
	if err = e.updateCandidate(result); err != nil {
		return err
	}
	result.Values[e.index].SoftCost.Set(&e.lower)
	return nil
}
