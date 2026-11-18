package opt

// Source: src/opt/maxres.cpp primal_dual_solver. It retains the source's
// alternating core/correction-set progress and clamps weighted lower estimates
// to the best published upper bound.

import "github.com/glycerine/g3/pkg/util/lbool"

func (e *maxResEngine) optimizePrimalDual(result *ArithmeticResult) (lbool.LBool, bool, error) {
	for iteration := 0; iteration < maxResIterationLimit; iteration++ {
		status, full, err := e.checkHillClimb(result)
		if err != nil {
			return status, false, err
		}
		// if (m.canceled()) return l_undef;
		if canceled, e2 := e.trail.manager.Canceled(); e2 != nil || canceled {
			return lbool.Undef, false, e2
		}
		if status == lbool.Undef {
			return status, false, nil
		}
		if status == lbool.True && full {
			if err = e.updateResult(result); err != nil {
				return lbool.Undef, false, err
			}
			if err := e.q().Copy(&e.lower, &e.upper); err != nil {
				return lbool.Undef, false, err
			}
			return lbool.False, true, nil
		}
		rawCore, err := e.solver.solver().UnsatCore()
		if err != nil {
			return lbool.Undef, false, err
		}
		var roundStatus lbool.LBool
		if roundStatus, err = e.processUnsatRound(rawCore, result); err != nil {
			return lbool.Undef, false, err
		}
		if roundStatus == lbool.Undef {
			return lbool.Undef, false, nil
		}
		if roundStatus == lbool.False {
			if err := e.q().Copy(&e.lower, &e.upper); err != nil {
				return lbool.Undef, false, err
			}
			return lbool.False, true, nil
		}
		e.clampPrimalDualLower()
	}
	return lbool.Undef, false, errMaxResState
}
