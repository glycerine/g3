package api

// Source: src/api/api_solver.cpp:76-85.

func (c *SMTContext) NewSolver() (*APISimpleSolver, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	// Z3_mk_solver creates only the API reference. Building the strategy
	// here consumes expression IDs before the client's first assertion.
	return &APISimpleSolver{owner: c.smtContextState, pendingStrategic: true}, nil
}
