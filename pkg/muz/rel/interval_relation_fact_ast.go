package rel

// Source: src/muz/rel/dl_interval_relation.cpp
// interval_relation::contains_fact.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

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
