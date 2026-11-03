package rel

// Source: src/muz/rel/dl_product_relation.cpp join_fn::init T_FULL completion
// for concrete table-only and interval-only input relations.

import "github.com/glycerine/g3/pkg/ast"

// NewSparseIntervalFiniteOriginal constructs the concrete evaluator owner for
// a bounded finite original predicate. Its interval component is a full,
// transparent zero-column sieve.
func NewSparseIntervalFiniteOriginal(manager *RelationManager, sorts []ast.ASTHandle, full bool) (*SparseIntervalSieveProduct, error) {
	finiteColumns, _, _, err := sparseIntervalProductColumns(manager, sorts)
	if err != nil {
		return nil, err
	}
	if len(sorts) == 0 || len(finiteColumns) != len(sorts) {
		return nil, ErrSparseIntervalProduct
	}
	return newSparseIntervalSieveProduct(manager, sorts, full, false)
}

// NewSparseIntervalArithmeticOriginal constructs the concrete evaluator owner
// for an Int/Real original predicate. Its finite component is a full,
// transparent zero-column sparse table.
func NewSparseIntervalArithmeticOriginal(manager *RelationManager, sorts []ast.ASTHandle, full bool) (*SparseIntervalSieveProduct, error) {
	finiteColumns, _, _, err := sparseIntervalProductColumns(manager, sorts)
	if err != nil {
		return nil, err
	}
	if len(sorts) == 0 || len(finiteColumns) != 0 {
		return nil, ErrSparseIntervalProduct
	}
	return newSparseIntervalSieveProduct(manager, sorts, full, false)
}

// FiniteSnapshot returns an owned exact sparse snapshot of a pure finite
// original. The product itself deliberately does not implement FiniteTable.
func (p *SparseIntervalSieveProduct) FiniteSnapshot() (*SparseTable, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if len(p.sorts) == 0 || len(p.finiteColumns) != len(p.sorts) {
		return nil, ErrSparseIntervalProduct
	}
	return p.finite.Clone()
}

// ArithmeticSnapshot returns an owned interval snapshot of a pure arithmetic
// original.
func (p *SparseIntervalSieveProduct) ArithmeticSnapshot() (*IntervalRelation, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if len(p.sorts) == 0 || len(p.finiteColumns) != 0 {
		return nil, ErrSparseIntervalProduct
	}
	return p.interval.inner.Clone()
}
