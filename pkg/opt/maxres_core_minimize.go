package opt

// Source: src/opt/maxres.cpp minimize_core, using the retained SMT assumption
// solver's deletion check for small cores.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

const maxResCoreMinimizeLimit = 32

func (e *maxResEngine) minimizeCore(core []ast.ASTHandle) ([]ast.ASTHandle, error) {
	limit := maxResCoreMinimizeLimit
	if len(core) > limit {
		return core, nil
	}
	answer := append([]ast.ASTHandle(nil), core...)
	for index := 0; index < len(answer); {
		trial := make([]ast.ASTHandle, 0, len(answer)-1)
		trial = append(trial, answer[:index]...)
		trial = append(trial, answer[index+1:]...)
		e.stats.CoreMinChecks++
		status, err := e.solver.backend.CheckSat(trial)
		if err != nil {
			return nil, err
		}
		switch status {
		case lbool.False:
			answer = trial
		case lbool.True:
			index++
		default:
			return nil, errMaxResState
		}
	}
	return answer, nil
}
