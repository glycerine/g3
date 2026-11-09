package opt

// Source: src/opt/maxres.cpp found_optimum and lower/upper loop guard.

func (e *maxResEngine) optimalByBounds() bool {
	return e.bestSet && e.lower.Cmp(&e.upper) >= 0
}

func (e *maxResEngine) publishOptimum(result *ArithmeticResult) error {
	if e == nil || result == nil || e.index < 0 || e.index >= len(result.Values) || !e.bestSet {
		return errMaxResState
	}
	// found_optimum recomputes m_lower from the soft constraints false in
	// the best model and sets m_upper to it; the core-driven lower bound can
	// overshoot that cost.
	if result.Model != nil {
		cost, _, _, err := e.solver.evaluateSoftDetailed(e.objective, result.Model)
		if err != nil {
			return err
		}
		e.lower.Set(&cost)
		e.upper.Set(&cost)
	}
	result.Values[e.index].SoftCost.Set(&e.lower)
	return nil
}
