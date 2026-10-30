package opt

// Source: src/opt/maxres.cpp mus_solver/primal_dual_solver l_false branches.

func (e *maxResEngine) processSearchUNSAT(result *ArithmeticResult) (bool, error) {
	proven, err := e.upperBoundProvesOptimal()
	if err != nil {
		return false, err
	}
	if proven {
		e.lower.Set(&e.upper)
		result.Values[e.index].SoftCost.Set(&e.upper)
		return true, nil
	}
	// Native always applies the core transformation and lower-bound update
	// before considering a correction-set pivot. The prerequisite batch's
	// standalone pivot helper selects before transformation, so do not feed it
	// the stale pre-core candidate here.
	e.correction = nil
	if err := e.processUnsatCore(result); err != nil {
		return false, err
	}
	e.clampPrimalDualLower()
	e.sortAssumptions()
	return false, nil
}
