package opt

// Source: src/opt/maxres.cpp mus_solver/primal_dual_solver l_true branches.

import "github.com/glycerine/g3/pkg/ast"

func (e *maxResEngine) processSearchSAT(result *ArithmeticResult, full bool) (bool, error) {
	if !full {
		return false, nil
	}
	// primal_dual_solver's get_current_correction_set(cs) runs
	// update_assignment on the same model again, which traces the
	// (unchanged) bounds (AUDIT.md FUZZ-F5; g3trace builds only).
	if e.strategy == maxResPrimalDual {
		if err := e.trace(); err != nil {
			return false, err
		}
	}
	if e.strategy == maxResPrimalDual && len(e.currentCorrection) != 0 {
		if err := e.processCorrection(append([]ast.ASTHandle(nil), e.currentCorrection...)); err != nil {
			return false, err
		}
		return false, nil
	}
	if e.strategy == maxResPrimalDual {
		// primal_dual_solver closes the remaining bound gap when the current
		// model has no correction set.
		e.foundFeasibleOptimum = result.Model != nil
		if err := e.q().Copy(&e.lower, &e.upper); err != nil {
			return false, err
		}
	}
	return true, result.Q.Copy(&result.Values[e.index].SoftCost, &e.lower)
}
