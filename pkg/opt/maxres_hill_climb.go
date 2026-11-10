package opt

// Source: src/opt/maxres.cpp check_sat_hill_climb, check_sat, next_index,
// sort_assumptions.

import (
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// sortedAssumptions is sort_assumptions on check_sat_hill_climb's local
// copy: m_asms keeps its order. (Go's stable sort; std::sort is not ported.)
func (e *maxResEngine) sortedAssumptions(asms []ast.ASTHandle) []ast.ASTHandle {
	out := append([]ast.ASTHandle(nil), asms...)
	sort.SliceStable(out, func(i, j int) bool {
		return e.weights[out[i]].Cmp(e.weights[out[j]]) > 0
	})
	return out
}

func (e *maxResEngine) nextIndex(asms []ast.ASTHandle, index int) int {
	if index < len(asms) {
		w := e.weights[asms[index]]
		index++
		for index < len(asms) && w.Cmp(e.weights[asms[index]]) == 0 {
			index++
		}
	}
	return index
}

// checkSat is maxres::check_sat: a satisfying model updates the assignment.
func (e *maxResEngine) checkSat(asms []ast.ASTHandle, result *ArithmeticResult) (lbool.LBool, error) {
	e.stats.Checks++
	status, err := e.solver.solver().CheckSat(asms)
	if err != nil || status != lbool.True {
		return status, err
	}
	return status, e.updateCandidate(result)
}

// checkSatHillClimb gives preference to cores with large minimal weights:
// it checks growing prefixes of the weight-sorted assumptions, each chunk
// at least a twentieth of them.
func (e *maxResEngine) checkSatHillClimb(asms1 []ast.ASTHandle, result *ArithmeticResult) (lbool.LBool, error) {
	if !e.config.hillClimb {
		return e.checkSat(asms1, result)
	}
	asms := e.sortedAssumptions(asms1)
	status := lbool.True
	index, last := 0, 0
	first := false
	for index < len(asms) && status == lbool.True {
		for !first && len(asms) > 20*(index-last) && index < len(asms) {
			index = e.nextIndex(asms, index)
		}
		first = false
		last = index
		var err error
		if status, err = e.checkSat(asms[:index], result); err != nil {
			return lbool.Undef, err
		}
	}
	return status, nil
}
