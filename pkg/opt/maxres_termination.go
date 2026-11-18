package opt

import "errors"

// Source: src/opt/maxres.cpp found_optimum and lower/upper loop guard.

func (e *maxResEngine) optimalByBounds() bool {
	return e.bestSet && e.cmp(&e.lower, &e.upper) >= 0
}

func (e *maxResEngine) publishOptimum(result *ArithmeticResult) error {
	if e == nil || result == nil || e.index < 0 || e.index >= len(result.Values) || !e.bestSet {
		return errMaxResState
	}
	// found_optimum recomputes m_lower from the soft constraints false in
	// the best model and sets m_upper to it; the core-driven lower bound can
	// overshoot that cost.
	if result.Model != nil {
		cost, satisfied, _, err := e.solver.evaluateSoftDetailed(e.objective, result.Model)
		q := e.q()
		if err == nil {
			err = errors.Join(q.Copy(&e.lower, &cost), q.Copy(&e.upper, &cost))
		}
		if err = errors.Join(err, q.Del(&cost), q.Del(&satisfied)); err != nil {
			return err
		}
	}
	return result.Q.Copy(&result.Values[e.index].SoftCost, &e.lower)
}
