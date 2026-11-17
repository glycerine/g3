package core

// Source: src/tactic/core/collect_occs.cpp/.h. The retained consumer creates a
// fresh collector and an empty result for each elim-uncnstr round.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

// collectSingleOccurrenceVariables returns borrowed constants in discovery
// order. The goal owns their lifetime. Maps are lookup-only; the ordered slice
// supplies traversal order to the consumer without depending on Go map order.
func collectSingleOccurrenceVariables(goal *tactic.Goal) ([]ast.ASTHandle, error) {
	if goal == nil || goal.Manager() == nil {
		return nil, ast.ErrASTHandle
	}
	m := goal.Manager()
	// expr_fast_mark1/2 in collect_occs: per-node mark bits, kept here in a
	// slice indexed by the node's dense slot (no node is freed during this
	// pass). Bit 1 is visited, bit 2 repeated. Go maps keyed by handle were
	// about 20% of lia_big_parse.
	var marks []uint8
	mark := func(term ast.ASTHandle) *uint8 {
		slot := int(term.Slot())
		if slot >= len(marks) {
			grown := make([]uint8, slot+1+slot/2)
			copy(grown, marks)
			marks = grown
		}
		return &marks[slot]
	}
	// markConstant records, at the first visit, that the node is an
	// uninterpreted constant, so a revisit needs no node read.
	const markVisited, markRepeated, markConstant = 1, 2, 4
	var variables []ast.ASTHandle
	type frame struct {
		term        ast.ASTHandle
		next, count uint32 // count: the app's argument count (read at push)
		quantifier  bool
	}
	var stack []frame
	visit := func(term ast.ASTHandle) (bool, error) {
		bits := mark(term)
		if *bits&markVisited != 0 {
			if *bits&markConstant != 0 {
				*bits |= markRepeated
			}
			return true, nil
		}
		kind, _, count, err := m.NodeShape(term)
		if err != nil {
			return false, err
		}
		if kind == ast.ASTApp && count == 0 {
			_, family, _, err := m.AppIdentity(term)
			if err != nil {
				return false, err
			}
			if family == ast.NullFamilyID {
				*bits |= markVisited | markConstant
				variables = append(variables, term)
				return true, nil
			}
		}
		*bits |= markVisited
		if kind == ast.ASTVar || kind == ast.ASTApp && count == 0 {
			return true, nil
		}
		if kind != ast.ASTApp && kind != ast.ASTQuantifier {
			return false, ast.ErrASTDomain
		}
		stack = append(stack, frame{term: term, count: count, quantifier: kind == ast.ASTQuantifier})
		return false, nil
	}
	size, err := goal.Size()
	if err != nil {
		return nil, err
	}
	for i := uint32(0); i < size; i++ {
		term, err := goal.Form(i)
		if err != nil {
			return nil, err
		}
		if _, err = visit(term); err != nil {
			return nil, err
		}
		for len(stack) != 0 {
			top := len(stack) - 1
			term = stack[top].term
			if stack[top].quantifier {
				// Source intentionally revisits the body after returning from
				// a compound child. visit sees its mark and retires this frame.
				body, err := m.GetQuantifierBody(term)
				if err != nil {
					return nil, err
				}
				stack[top].next++
				done, err := visit(body)
				if err != nil {
					return nil, err
				}
				if done {
					stack = stack[:top]
				}
				continue
			}
			if stack[top].next == stack[top].count {
				stack = stack[:top]
				continue
			}
			child, err := m.GetArg(term, stack[top].next)
			if err != nil {
				return nil, err
			}
			stack[top].next++
			if _, err = visit(child); err != nil {
				return nil, err
			}
		}
	}
	result := variables[:0]
	for _, variable := range variables {
		if *mark(variable)&markRepeated == 0 {
			result = append(result, variable)
		}
	}
	return result, nil
}
