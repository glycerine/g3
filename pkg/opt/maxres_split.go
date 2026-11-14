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
	// remove_core(core): only m_asms loses the members. m_asm2weight keeps
	// every weight it was given, so a later core of the same batch that
	// shares a member with a correction set processed in between
	// (process_unsat's pivot) still finds its weight (AUDIT.md FUZZ_J-F2).
	e.active = removeSoft(core, e.active)
	// split_core: if (core.empty()) return rational(0);
	if len(core) == 0 {
		return new(big.Rat), nil
	}
	weights := make([]*big.Rat, len(core))
	var minimum *big.Rat
	for i, expression := range core {
		weight := e.weights[expression]
		if weight == nil {
			return nil, errMaxResState
		}
		weights[i] = new(big.Rat).Set(weight)
		if minimum == nil || weight.Cmp(minimum) < 0 {
			minimum = new(big.Rat).Set(weight)
		}
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
