package rel

// Test-only IntervalRelation accessors: the datalog engine reaches the
// interval relation through its plugin functors and AST facts only.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
)

// IsFull mirrors vector_relation::is_full for interval columns.
func (r *IntervalRelation) IsFull() bool {
	if r == nil || r.values == nil {
		return false
	}
	for i := range r.values.columns {
		lower, _, upper, _ := r.values.columns[r.values.find(uint32(i))].Bounds()
		if lower != nil || upper != nil {
			return false
		}
	}
	return true
}

// Bounds returns copies of the column's endpoints; nil denotes infinity.
func (r *IntervalRelation) Bounds(column uint32) (*big.Rat, bool, *big.Rat, bool, error) {
	if r == nil || r.values == nil || uint64(column) >= uint64(len(r.values.columns)) {
		return nil, false, nil, false, ast.ErrASTDomain
	}
	lo, loOpen, hi, hiOpen := r.values.columns[r.values.find(column)].Bounds()
	return lo, loOpen, hi, hiOpen, nil
}

func (r *IntervalRelation) FilterIdentical(first, second uint32) error {
	return r.FilterIdenticalColumns([]uint32{first, second})
}

// AddFact joins a concrete rational tuple into the interval abstraction.
func (r *IntervalRelation) AddFact(values []*big.Rat) error {
	if r == nil || r.values == nil || len(values) != len(r.values.columns) {
		return ast.ErrASTDomain
	}
	fact := newIntervalColumns(len(values), false)
	for i, value := range values {
		if value == nil {
			return ast.ErrASTDomain
		}
		fact.columns[i] = smtcore.NewRelationInterval(value, false, value, false)
	}
	_, err := unionIntervalColumns(r.values, fact, false)
	return err
}

func (r *IntervalRelation) ContainsFact(values []*big.Rat) bool {
	if r == nil || r.values == nil || len(values) != len(r.values.columns) {
		return false
	}
	// The C++ contains_fact path reads reset column payloads even when the
	// relation's empty flag is set, so do not short-circuit on r.Empty().
	for i, value := range values {
		root := r.values.find(uint32(i))
		if value == nil || values[root] == nil || value.Cmp(values[root]) != 0 || !r.values.columns[root].Contains(value) {
			return false
		}
	}
	return true
}

// Arity returns the concrete vector-relation column count.
func (r *IntervalRelation) Arity() (uint32, error) {
	if r == nil || r.values == nil {
		return 0, ast.ErrASTDomain
	}
	return uint32(len(r.values.columns)), nil
}
