package opt

// Source: src/opt/maxres.cpp process_unsat(vector<exprs> const&).

import "github.com/glycerine/g3/pkg/ast"

// process_unsat(cores): each core in turn; the only pivot is the one
// process_unsat(core) makes after the core (pivotAfterCore).
func (e *maxResEngine) processCoreBatch(cores [][]ast.ASTHandle) error {
	for _, core := range cores {
		if err := e.processCore(core); err != nil {
			return err
		}
		e.clampPrimalDualLower()
		if err := e.pivotAfterCore(core); err != nil {
			return err
		}
	}
	return nil
}
