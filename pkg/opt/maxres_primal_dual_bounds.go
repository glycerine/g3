package opt

// Source: src/opt/maxres.cpp process_unsat primal-dual lower-bound clamp.

func (e *maxResEngine) clampPrimalDualLower() {
	if e.strategy == maxResPrimalDual && e.bestSet && e.cmp(&e.lower, &e.upper) > 0 {
		e.noteRat(e.q().Copy(&e.lower, &e.upper))
	}
}
