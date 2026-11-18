package rel

// Source: src/muz/rel/dl_interval_relation.cpp
// interval_relation::contains_fact.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// AddFactExpressions constructs the source full interval tuple, filters each
// column by its AST equality, then unions that abstraction into the receiver.
func (r *IntervalRelation) AddFactExpressions(manager *ast.ASTManager, sorts, values []ast.ASTHandle) (resultErr error) {
	if r == nil || r.values == nil || manager == nil || len(sorts) != len(values) || len(values) != len(r.values.columns) || !CanHandleIntervalSorts(manager, sorts) {
		return ast.ErrASTDomain
	}
	fact := NewIntervalFull(uint32(len(values)))
	for i, value := range values {
		sort, err := manager.GetSort(value)
		if err != nil || sort != sorts[i] {
			if err == nil {
				err = ast.ErrASTDomain
			}
			return err
		}
		// app_ref eq(m); eq = m.mk_eq(m.mk_var(i, m.get_sort(e)), e): the
		// variable is held only by the equality, which is released at the
		// end of each iteration.
		variable, err := manager.MkVar(uint32(i), sorts[i])
		if err != nil {
			return err
		}
		equality, err := manager.MkEq(variable, value)
		if err != nil {
			return err
		}
		eq, err := manager.NewRef(equality)
		if err != nil {
			return err
		}
		err = fact.FilterInterpreted(manager, equality)
		if err = errors.Join(err, manager.DelRef(&eq)); err != nil {
			return err
		}
	}
	_, resultErr = r.Union(fact, false)
	return resultErr
}

// ContainsFactExpressions applies the source's AST-valued membership check.
// Non-numerals are accepted conservatively, while members of one equality
// class must be represented by the same AST handle.
func (r *IntervalRelation) ContainsFactExpressions(manager *ast.ASTManager, values []ast.ASTHandle) (bool, error) {
	if r == nil || r.values == nil || manager == nil || len(values) != len(r.values.columns) {
		return false, ast.ErrASTDomain
	}
	for i, value := range values {
		root := r.values.find(uint32(i))
		if value != values[root] {
			return false, nil
		}
		interval := r.values.columns[root]
		lower, _, upper, _ := interval.Bounds()
		if lower == nil && upper == nil {
			continue
		}
		text, _, matched, err := manager.ArithmeticNumeralText(value)
		if err != nil {
			return false, err
		}
		if !matched {
			continue
		}
		number, ok := new(big.Rat).SetString(text)
		if !ok {
			return false, ast.ErrASTDomain
		}
		if !interval.Contains(number) {
			return false, nil
		}
	}
	return true, nil
}
