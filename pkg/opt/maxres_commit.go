package opt

// Source: src/opt/maxres.cpp commit_assignment.

// commitAssignment asserts m_defs and m_asms once a feasible optimum was
// found; otherwise there is only a single assignment to the soft
// constraints and nothing is asserted.
func (e *maxResEngine) commitAssignment() error {
	if e == nil || e.solver == nil {
		return errMaxResState
	}
	if !e.foundFeasibleOptimum {
		return nil
	}
	backend := e.solver.solver()
	for _, f := range e.defs {
		if err := backend.Assert(f); err != nil {
			return err
		}
	}
	for _, a := range e.active {
		if err := backend.Assert(a); err != nil {
			return err
		}
	}
	return nil
}
