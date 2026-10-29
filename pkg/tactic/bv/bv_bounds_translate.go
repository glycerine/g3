package bv

// Source: src/tactic/bv/bv_bounds_tactic.cpp simplifier::translate.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic/core"
)

func (s *BVBoundsSimplifier) Translate(m *ast.ASTManager) (core.ContextSimplifier, error) {
	n := NewBVBoundsSimplifier(m, s.context)
	n.propagateEq = s.propagateEq
	return n, nil
}
