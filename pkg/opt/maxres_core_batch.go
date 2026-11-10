package opt

// Source: src/opt/maxres.cpp get_cores, remove_soft.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// removeSoft is remove_soft: core members leave asms, each replaced by the
// last element.
func removeSoft(core, asms []ast.ASTHandle) []ast.ASTHandle {
	members := make(map[ast.ASTHandle]struct{}, len(core))
	for _, expression := range core {
		members[expression] = struct{}{}
	}
	for i := 0; i < len(asms); i++ {
		if _, found := members[asms[i]]; found {
			asms[i] = asms[len(asms)-1]
			asms = asms[:len(asms)-1]
			i--
		}
	}
	return asms
}

// collectCoreBatch is get_cores, entered with the solver unsatisfiable
// under the assumptions and first its unsat core.
func (e *maxResEngine) collectCoreBatch(first []ast.ASTHandle, result *ArithmeticResult) ([][]ast.ASTHandle, lbool.LBool, error) {
	asms := append([]ast.ASTHandle(nil), e.active...)
	var cores [][]ast.ASTHandle
	raw := first
	status := lbool.False
	for status == lbool.False {
		core, err := e.filterCore(raw)
		if err != nil {
			return nil, lbool.Undef, err
		}
		if err = e.updateMUSModel(result); err != nil {
			return nil, lbool.Undef, err
		}
		core, status, err = e.minimizeCore(core)
		if err != nil {
			return nil, lbool.Undef, err
		}
		if status != lbool.True {
			break
		}
		if len(core) == 0 {
			e.lower.Set(&e.upper)
			return nil, lbool.True, nil
		}
		cores = append(cores, core)
		if e.coreBatchLimitReached(len(cores), len(core)) {
			break
		}
		asms = removeSoft(core, asms)
		if status, err = e.checkSatHillClimb(asms, result); err != nil {
			return nil, lbool.Undef, err
		}
		if status == lbool.False {
			if raw, err = e.solver.solver().UnsatCore(); err != nil {
				return nil, lbool.Undef, err
			}
		}
	}
	return cores, status, nil
}
