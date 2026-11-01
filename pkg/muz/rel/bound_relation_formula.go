package rel

// Source: src/muz/rel/dl_bound_relation.cpp bound_relation::to_formula.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// ToFormula builds the source conjunction over column variables. The returned
// AST reference is owned by the caller.
func (r *BoundRelation) ToFormula(m *ast.ASTManager, sorts []ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	if r == nil || m == nil || len(sorts) != len(r.columns) {
		return result, ast.ErrASTDomain
	}
	var roots []ast.ASTRef
	owned := false
	defer func() {
		for i := len(roots) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&roots[i]))
		}
		if resultErr != nil && owned {
			resultErr = errors.Join(resultErr, m.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	variables := make([]ast.ASTHandle, len(sorts))
	for i, sort := range sorts {
		variable, err := m.MkVar(uint32(i), sort)
		if err != nil {
			return result, err
		}
		ref, err := m.NewRef(variable)
		if err != nil {
			return result, err
		}
		roots = append(roots, ref)
		variables[i] = variable
	}
	var conjuncts []ast.ASTHandle
	add := func(h ast.ASTHandle, err error) error {
		if err != nil {
			return err
		}
		ref, err := m.NewRef(h)
		if err != nil {
			return err
		}
		roots = append(roots, ref)
		conjuncts = append(conjuncts, h)
		return nil
	}
	for i := range r.columns {
		root := r.find(uint32(i))
		if root != uint32(i) {
			if err := add(m.MkEq(variables[i], variables[root])); err != nil {
				return result, err
			}
			continue
		}
		strict, weak, err := r.Bounds(uint32(i))
		if err != nil {
			return result, err
		}
		for _, j := range strict {
			if uint64(j) >= uint64(len(variables)) {
				return result, ast.ErrASTDomain
			}
			if err = add(m.MkArithmeticLT(variables[i], variables[j])); err != nil {
				return result, err
			}
		}
		for _, j := range weak {
			if uint64(j) >= uint64(len(variables)) {
				return result, ast.ErrASTDomain
			}
			if err = add(m.MkArithmeticLE(variables[i], variables[j])); err != nil {
				return result, err
			}
		}
	}
	result, resultErr = m.NewRef(ast.ASTHandle{})
	if resultErr != nil {
		return result, resultErr
	}
	owned = true
	resultErr = rewriter.SimplifyBooleanGateDefault(m, ast.ASTOpAnd, conjuncts, result)
	return result, resultErr
}
