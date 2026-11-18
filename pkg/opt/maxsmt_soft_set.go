package opt

// Source: src/opt/maxsmt.cpp maxsmt::add and maxsmt_solver_base ownership.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// weightedSoftSet owns normalized, unique soft formulas and their rational
// weights until Release transfers them into ArithmeticSolver's objective.
type weightedSoftSet struct {
	manager *ast.ASTManager
	terms   []ast.ASTRef
	weights []numeral.Rational
	index   map[ast.ASTHandle]int
	offset  numeral.Rational
	closed  bool
}

func newWeightedSoftSet(manager *ast.ASTManager) (*weightedSoftSet, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	return &weightedSoftSet{manager: manager, index: make(map[ast.ASTHandle]int), offset: numeral.NewRational()}, nil
}

func (s *weightedSoftSet) Len() int {
	if s == nil || s.closed {
		return 0
	}
	return len(s.terms)
}
