package opt

// Source: src/opt/maxsmt.cpp maxsmt_solver_base::find_mutexes and
// src/opt/maxres.cpp init_local.

import (
	"fmt"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *maxResEngine) preprocessMutexes(objective *arithmeticObjective) ([]maxResWeightedTerm, error) {
	expressions := make([]ast.ASTHandle, len(objective.softTerms))
	newSoft := newSoftWeightMap(e.solver.manager)
	for i := range objective.softTerms {
		expression, err := e.solver.manager.RefGet(objective.softTerms[i])
		if err != nil {
			return nil, err
		}
		expressions[i] = expression
		// maxsmt::add already merged duplicate soft constraints.
		weight := new(big.Rat).Set(objective.softWeights[i])
		old, err := newSoft.find(expression)
		if err != nil {
			return nil, err
		}
		if old != nil {
			weight.Add(weight, old)
		}
		if err = newSoft.insert(expression, weight); err != nil {
			return nil, err
		}
	}
	status, mutexes, err := e.solver.solver().FindMutexes(expressions)
	if err != nil {
		return nil, fmt.Errorf("maxres mutex discovery: %w", err)
	}
	if status != lbool.True {
		return nil, errMaxResState
	}
	for _, mutex := range mutexes {
		if err = e.transformMutex(mutex, newSoft); err != nil {
			return nil, fmt.Errorf("maxres mutex transform: %w", err)
		}
	}
	var terms []maxResWeightedTerm
	for _, term := range newSoft.entries() {
		if term.weight.Sign() > 0 {
			terms = append(terms, term)
		}
	}
	return terms, nil
}
