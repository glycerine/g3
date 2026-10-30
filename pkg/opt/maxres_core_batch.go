package opt

// Source: src/opt/maxres.cpp get_cores: extract disjoint cores from one
// unsatisfiable round before applying MaxRes transformations.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func removeMaxResCoreMembers(values, core []ast.ASTHandle) []ast.ASTHandle {
	members := make(map[ast.ASTHandle]struct{}, len(core))
	for _, expression := range core {
		members[expression] = struct{}{}
	}
	answer := values[:0]
	for _, expression := range values {
		if _, found := members[expression]; !found {
			answer = append(answer, expression)
		}
	}
	return answer
}

func (e *maxResEngine) collectCoreBatch(first []ast.ASTHandle, result *ArithmeticResult) ([][]ast.ASTHandle, error) {
	core, err := e.filterCore(first)
	if err != nil {
		return nil, err
	}
	core, err = e.minimizeCore(core)
	if err != nil {
		return nil, err
	}
	cores := [][]ast.ASTHandle{core}
	remaining := removeMaxResCoreMembers(append([]ast.ASTHandle(nil), e.active...), core)
	for len(remaining) != 0 && !e.coreBatchLimitReached(len(cores), len(core)) {
		e.stats.Checks++
		status, err := e.solver.backend.CheckSat(remaining)
		if err != nil {
			return nil, err
		}
		switch status {
		case lbool.True:
			if err = e.updateCandidate(result); err != nil {
				return nil, err
			}
			return cores, nil
		case lbool.False:
			raw, err := e.solver.backend.UnsatCore()
			if err != nil {
				return nil, err
			}
			core, err = e.filterCore(raw)
			if err != nil {
				return nil, err
			}
			core, err = e.minimizeCore(core)
			if err != nil {
				return nil, err
			}
			cores = append(cores, core)
			remaining = removeMaxResCoreMembers(remaining, core)
		default:
			return nil, errMaxResState
		}
	}
	return cores, nil
}
