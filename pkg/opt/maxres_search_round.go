package opt

// Source: src/opt/maxres.cpp process_unsat/get_cores round orchestration.

import "github.com/glycerine/g3/pkg/ast"

func (e *maxResEngine) processUnsatRound(raw []ast.ASTHandle, result *ArithmeticResult) error {
	cores, err := e.collectCoreBatch(raw, result)
	if err != nil {
		return err
	}
	return e.processCoreBatch(cores)
}
