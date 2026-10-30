package opt

// Source: src/opt/sortmax.cpp psort_nw invocation with concrete callbacks.

import "github.com/glycerine/g3/pkg/ast"

func (e *sortMaxEncoder) network(inputs []ast.ASTHandle) ([]ast.ASTHandle, error) {
	outputs := make([]ast.ASTHandle, 0, len(inputs))
	for _, input := range inputs {
		carry := input
		next := make([]ast.ASTHandle, len(outputs)+1)
		for i, previous := range outputs {
			high, low, err := e.comparator(previous, carry)
			if err != nil {
				return nil, err
			}
			next[i], carry = high, low
		}
		next[len(outputs)] = carry
		outputs = next
	}
	return outputs, nil
}
