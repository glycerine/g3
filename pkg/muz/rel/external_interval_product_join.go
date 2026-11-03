package rel

// Source: src/muz/rel/dl_product_relation.cpp join_fn for aligned product
// specifications, specialized to external_relation plus interval_relation.

import "github.com/glycerine/g3/pkg/ast"

// JoinExternalIntervalProducts joins corresponding external and interval
// components and returns their aligned product. The source result signature is
// the concatenation of both input signatures; join columns constrain tuples
// but are not projected.
func JoinExternalIntervalProducts(left, right *ExternalIntervalProduct, leftColumns, rightColumns []uint32) (*ExternalIntervalProduct, error) {
	if err := left.open(); err != nil {
		return nil, err
	}
	if err := right.open(); err != nil {
		return nil, err
	}
	if left.plugin != right.plugin || len(left.kinds) != len(right.kinds) || len(leftColumns) != len(rightColumns) {
		return nil, ErrTableRelation
	}
	for i := range left.kinds {
		if left.kinds[i] != right.kinds[i] {
			return nil, ErrTableRelation
		}
	}
	for i, leftColumn := range leftColumns {
		rightColumn := rightColumns[i]
		if uint64(leftColumn) >= uint64(len(left.sorts)) || uint64(rightColumn) >= uint64(len(right.sorts)) {
			return nil, ast.ErrASTDomain
		}
		if left.sorts[leftColumn] != right.sorts[rightColumn] {
			return nil, ast.ErrASTDomain
		}
	}
	interval, err := JoinIntervalRelations(left.interval, right.interval, leftColumns, rightColumns)
	if err != nil {
		return nil, err
	}
	external, err := JoinExternalRelations(left.external, right.external, leftColumns, rightColumns)
	if err != nil {
		return nil, err
	}
	return left.transformed(external, interval), nil
}
