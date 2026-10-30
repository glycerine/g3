package opt

// Source: src/opt/maxres.cpp get_cores, process_unsat and correction pivot.

import "fmt"

func (e *maxResEngine) processUnsatCore(result *ArithmeticResult) error {
	core, err := e.solver.backend.UnsatCore()
	if err != nil {
		return fmt.Errorf("maxres unsat core: %w", err)
	}
	if err = e.processUnsatRound(core, result); err != nil {
		return fmt.Errorf("maxres process core round: %w", err)
	}
	return nil
}
