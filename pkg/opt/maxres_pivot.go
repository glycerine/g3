package opt

// Source: src/opt/maxres.cpp process_unsat correction-set pivot.

import "github.com/glycerine/g3/pkg/ast"

func (e *maxResEngine) pivotCorrection(core []ast.ASTHandle) ([]ast.ASTHandle, bool) {
	if !e.config.pivotOnCorrectionSet || len(e.correction) == 0 {
		return nil, false
	}
	active := make([]ast.ASTHandle, 0, len(e.correction))
	seen := make(map[ast.ASTHandle]struct{}, len(e.correction))
	for _, expression := range e.correction {
		if e.weights[expression] == nil {
			continue
		}
		if _, duplicate := seen[expression]; duplicate {
			continue
		}
		seen[expression] = struct{}{}
		active = append(active, expression)
	}
	if len(active) == 0 || len(active) >= len(core) {
		return nil, false
	}
	e.stats.Pivots++
	return active, true
}
