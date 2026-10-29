package api

// Source: src/api/api_algebraic.cpp:353-387.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

// AlgebraicRoots isolates the roots in variable len(assignments), after
// assigning variables 0..len(assignments)-1 in source order.
func (c *SMTContext) AlgebraicRoots(expression ast.ASTHandle, assignments []ast.ASTHandle) (result []ast.ASTHandle, resultErr error) {
	manager, err := c.Manager()
	if err != nil {
		return nil, err
	}
	if uint64(len(assignments)) >= math.MaxUint32 {
		return nil, arena.ErrBounds
	}
	p, err := newAPIAlgebraicPolynomial(manager, expression)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.close()) }()
	maxVariable, err := p.maxVariable()
	if err != nil {
		return nil, err
	}
	if maxVariable >= uint32(len(assignments))+1 {
		return nil, ast.ErrASTDomain
	}
	values, err := newAPIAlgebraicAssignments(manager, assignments)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, values.close()) }()
	algebraic, err := manager.ArithmeticAlgebraicManager()
	if err != nil {
		return nil, err
	}
	roots, err := algebraic.NewVector()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, roots.Close()) }()
	if err = algebraic.IsolateMultivariateRoots(p.manager, p.value, &values, roots); err != nil {
		return nil, err
	}
	result = make([]ast.ASTHandle, roots.Len())
	for i := range result {
		value, e := roots.Borrow(uint32(i))
		if e != nil {
			return nil, e
		}
		result[i], e = makeAPIAlgebraicResult(manager, algebraic, value)
		if e != nil {
			return nil, e
		}
	}
	return result, nil
}
