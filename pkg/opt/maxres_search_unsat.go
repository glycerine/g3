package opt

// Source: src/opt/maxres.cpp mus_solver/primal_dual_solver l_false branches.

func (e *maxResEngine) processSearchUNSAT(result *ArithmeticResult) error {
	// Native always applies the core transformation and lower-bound update
	// before considering a correction-set pivot. The prerequisite batch's
	// standalone pivot helper selects before transformation, so do not feed it
	// the stale pre-core candidate here.
	e.correction = nil
	if err := e.processUnsatCore(result); err != nil {
		return err
	}
	e.clampPrimalDualLower()
	e.sortAssumptions()
	return nil
}
