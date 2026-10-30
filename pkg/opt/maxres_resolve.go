package opt

// Source: src/opt/maxres.cpp max_resolve.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (e *maxResEngine) maxResolve(core []ast.ASTHandle, weight *big.Rat) error {
	if len(core) == 0 || weight == nil || weight.Sign() <= 0 {
		return errMaxResState
	}
	clauses, err := maxResResolutionClauses(&e.owner, core)
	if err != nil {
		return err
	}
	for _, clause := range clauses {
		proxy, err := e.freshProxy(clause, false)
		if err != nil {
			return err
		}
		if err = e.addAssumption(proxy, weight); err != nil {
			return err
		}
		e.stats.Resolutions++
	}
	return nil
}
