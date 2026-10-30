package opt

// Source: src/opt/maxres.cpp mus_solver/primal_dual_solver l_false branches.

import "github.com/glycerine/g3/pkg/util/lbool"

func (e *maxResEngine) processSearchUNSAT(result *ArithmeticResult) (lbool.LBool, error) {
	// Native always applies the core transformation and lower-bound update
	// before considering a correction-set pivot. The prerequisite batch's
	// standalone pivot helper selects before transformation, so do not feed it
	// the stale pre-core candidate here.
	e.correction = nil
	status, err := e.processUnsatCore(result)
	if err != nil {
		return lbool.Undef, err
	}
	if status == lbool.False {
		e.lower.Set(&e.upper)
		return lbool.False, nil
	}
	if status != lbool.True {
		return status, nil
	}
	e.clampPrimalDualLower()
	e.sortAssumptions()
	return lbool.True, nil
}
