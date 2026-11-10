package opt

// Source: src/opt/maxsmt.cpp maxsmt_solver_base::process_mutex, exact rational
// specialization used by MaxRes.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (e *maxResEngine) transformMutex(mutex []ast.ASTHandle, newSoft *softWeightMap) error {
	if len(mutex) <= 1 {
		return nil
	}
	sorted := append([]ast.ASTHandle(nil), mutex...)
	byWeight := make(map[ast.ASTHandle]*big.Rat, len(sorted))
	for _, expression := range sorted {
		weight, err := newSoft.find(expression)
		if err != nil {
			return err
		}
		if weight == nil {
			return errMaxResState
		}
		byWeight[expression] = weight
	}
	sortMaxResMutex(sorted, byWeight)
	weights := make([]big.Rat, len(sorted))
	for i, expression := range sorted {
		weights[i].Set(byWeight[expression])
		if err := newSoft.remove(expression); err != nil {
			return err
		}
	}
	var weight big.Rat
	for i := len(sorted); i > 0; {
		i--
		soft, err := e.owner.hold(e.solver.manager.MkOrArgs(sorted[:i+1]))
		if err != nil {
			return err
		}
		w := new(big.Rat).Set(&weights[i])
		weight.Sub(w, &weight)
		e.lower.Add(&e.lower, new(big.Rat).Mul(&weight, new(big.Rat).SetInt64(int64(i))))
		if err = newSoft.insert(soft, &weight); err != nil {
			return err
		}
		e.stats.MutexTerms++
		for i > 0 && weights[i-1].Cmp(w) == 0 {
			i--
		}
		weight.Set(w)
	}
	e.stats.Mutexes++
	return nil
}
