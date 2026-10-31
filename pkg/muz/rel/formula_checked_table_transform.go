package rel

// Source: src/muz/rel/check_relation.cpp, relation transformations.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func (r *FormulaCheckedTable) substitute(formula ast.ASTHandle, bindings []ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	subst, err := rewriter.NewVarSubstitution(r.manager, false)
	if subst != nil {
		defer func() {
			resultErr = errors.Join(resultErr, subst.Close())
			if resultErr != nil && result != (ast.ASTRef{}) {
				resultErr = errors.Join(resultErr, r.manager.DelRef(&result))
				result = ast.ASTRef{}
			}
		}()
	}
	if err != nil {
		return result, err
	}
	result, err = r.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	resultErr = subst.Substitute(formula, bindings, result)
	return result, resultErr
}

// RenameCycle applies the concrete table rename and checks the source formula
// under the corresponding variable permutation.
func (r *FormulaCheckedTable) RenameCycle(cycle []int) (_ *FormulaCheckedTable, resultErr error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if len(cycle) < 2 {
		return nil, ast.ErrASTDomain
	}
	if _, err := tableRelationCyclePermutation(len(r.table.sorts), cycle); err != nil {
		return nil, err
	}
	table, err := r.table.RenameCycle(cycle)
	if err != nil {
		return nil, err
	}
	result, err := NewFormulaCheckedTable(table, r.config)
	if err != nil {
		return nil, errors.Join(err, table.Close())
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	permutation := make([]int, len(r.table.sorts))
	for i := range permutation {
		permutation[i] = i
	}
	for i, column := range cycle {
		permutation[cycle[(i+1)%len(cycle)]] = column
	}
	bindings := make([]ast.ASTHandle, len(permutation))
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, r.manager.DelRef(&refs[i]))
		}
	}()
	for i, dest := range permutation {
		if result.table.sorts[dest] != r.table.sorts[i] {
			return nil, ast.ErrASTDomain
		}
		variable, err := r.manager.MkVar(uint32(dest), r.table.sorts[i])
		if err != nil {
			return nil, err
		}
		ref, err := r.manager.NewRef(variable)
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
		bindings[i] = variable
	}
	sourceFormula, err := r.table.ToFormula()
	if err != nil {
		return nil, err
	}
	refs = append(refs, sourceFormula)
	source, err := r.manager.RefGet(sourceFormula)
	if err != nil {
		return nil, err
	}
	permuted, err := r.substitute(source, bindings)
	if err != nil {
		return nil, err
	}
	refs = append(refs, permuted)
	want, err := r.manager.RefGet(permuted)
	if err != nil {
		return nil, err
	}
	got, err := r.manager.RefGet(result.formula)
	if err != nil {
		return nil, err
	}
	if err = result.verifyGrounded(want, got, false); err != nil {
		return nil, fmt.Errorf("permutation: %w", err)
	}
	return result, nil
}
