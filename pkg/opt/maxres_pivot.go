package opt

// Source: src/opt/maxres.cpp:497-516 process_unsat correction-set pivot.

import "github.com/glycerine/g3/pkg/ast"

// pivotAfterCore mirrors process_unsat's ordering: transform the core and
// update the lower bound first, then consult the best correction-set model.
func (e *maxResEngine) pivotAfterCore(core []ast.ASTHandle) error {
	if e.correctionModel == nil {
		return nil
	}
	if e.correctionSize > 0 {
		e.correctionSize--
	}
	if !e.config.pivotOnCorrectionSet || len(e.solver.objectives) != 1 || e.correctionSize >= len(core) {
		return nil
	}
	correction, err := e.correctionSet(e.correctionModel)
	if err != nil {
		return err
	}
	e.correctionSize = len(correction)
	if len(correction) >= len(core) {
		return nil
	}
	e.stats.Pivots++
	return e.processCorrection(correction)
}
