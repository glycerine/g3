package opt

// Source: src/opt/maxres.cpp primal_dual_solver. It retains the source's
// alternating core/correction-set progress and clamps weighted lower estimates
// to the best published upper bound.

import "github.com/glycerine/g3/pkg/util/lbool"

func (e *maxResEngine) optimizePrimalDual(result *ArithmeticResult) (lbool.LBool, bool, error) {
	if err := e.trySLSCandidate(result); err != nil {
		return lbool.Undef, false, err
	}
	for iteration := 0; iteration < maxResIterationLimit; iteration++ {
		status, full, err := e.checkHillClimb(result)
		if err != nil || status == lbool.Undef {
			return status, false, err
		}
		if status == lbool.True && full {
			if err = e.updateResult(result); err != nil {
				return lbool.Undef, false, err
			}
			e.lower.Set(&e.upper)
			return lbool.False, true, nil
		}
		rawCore, err := e.solver.backend.UnsatCore()
		if err != nil {
			return lbool.Undef, false, err
		}
		proven, err := e.upperBoundProvesOptimal()
		if err != nil {
			return lbool.Undef, false, err
		}
		if proven {
			e.lower.Set(&e.upper)
			return lbool.False, true, nil
		}
		if err = e.processUnsatRound(rawCore, result); err != nil {
			return lbool.Undef, false, err
		}
		e.clampPrimalDualLower()
	}
	return lbool.Undef, false, errMaxResState
}
