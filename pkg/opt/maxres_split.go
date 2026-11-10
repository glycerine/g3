package opt

// Source: src/opt/maxres.cpp remove_core and split_core.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// splitCore is remove_core(core) followed by split_core(core): the core
// leaves m_asms (remove_soft swaps with the last element), and members
// heavier than the minimum come back as fresh assumptions with the excess.
func (e *maxResEngine) splitCore(core []ast.ASTHandle) (*big.Rat, error) {
	if len(core) == 0 {
		return nil, errMaxResState
	}
	weights := make([]*big.Rat, len(core))
	var minimum *big.Rat
	for i, expression := range core {
		weight := e.weights[expression]
		if weight == nil || weight.Sign() <= 0 {
			return nil, errMaxResState
		}
		weights[i] = new(big.Rat).Set(weight)
		if minimum == nil || weight.Cmp(minimum) < 0 {
			minimum = new(big.Rat).Set(weight)
		}
	}
	e.active = removeSoft(core, e.active)
	for _, expression := range core {
		delete(e.weights, expression)
	}
	for i, expression := range core {
		if weights[i].Cmp(minimum) > 0 {
			if err := e.addAssumption(expression, new(big.Rat).Sub(weights[i], minimum)); err != nil {
				return nil, err
			}
		}
	}
	return minimum, nil
}
