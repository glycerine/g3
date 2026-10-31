package rel

// Source: src/muz/rel/dl_interval_relation.cpp and the concrete interval
// instantiation of dl_vector_relation.h.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
)

// IntervalRelation is the concrete rational interval abstraction.
type IntervalRelation struct{ values *intervalColumns }

func NewIntervalEmpty(arity uint32) *IntervalRelation {
	return &IntervalRelation{newIntervalColumns(int(arity), true)}
}
func NewIntervalFull(arity uint32) *IntervalRelation {
	return &IntervalRelation{newIntervalColumns(int(arity), false)}
}
func (r *IntervalRelation) Empty() bool { return r == nil || r.values == nil || r.values.empty }

// Bounds returns copies of the column's endpoints; nil denotes infinity.
func (r *IntervalRelation) Bounds(column uint32) (*big.Rat, bool, *big.Rat, bool, error) {
	if r == nil || r.values == nil || uint64(column) >= uint64(len(r.values.columns)) {
		return nil, false, nil, false, ast.ErrASTDomain
	}
	lo, loOpen, hi, hiOpen := r.values.columns[r.values.find(column)].Bounds()
	return lo, loOpen, hi, hiOpen, nil
}

func (r *IntervalRelation) FilterEqual(column uint32, value *big.Rat) error {
	if r == nil || r.values == nil || value == nil {
		return ast.ErrASTDomain
	}
	return r.values.intersect(column, smtcore.NewRelationInterval(value, false, value, false))
}

func (r *IntervalRelation) FilterIdentical(first, second uint32) error {
	if r == nil || r.values == nil {
		return ast.ErrASTDomain
	}
	return r.values.equate(first, second)
}

func (r *IntervalRelation) FilterInterpreted(manager *ast.ASTManager, condition ast.ASTHandle) error {
	if r == nil || r.values == nil || manager == nil {
		return ast.ErrASTDomain
	}
	return r.values.filterInterpreted(manager, condition)
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
	if r.Empty() || len(values) != len(r.values.columns) {
		return false
	}
	for i, value := range values {
		root := r.values.find(uint32(i))
		if value == nil || values[root] == nil || value.Cmp(values[root]) != 0 || !r.values.columns[root].Contains(value) {
			return false
		}
	}
	return true
}

func (r *IntervalRelation) Union(source *IntervalRelation, widen bool) (bool, error) {
	if r == nil || source == nil {
		return false, ast.ErrASTDomain
	}
	return unionIntervalColumns(r.values, source.values, widen)
}

func JoinIntervalRelations(left, right *IntervalRelation, leftColumns, rightColumns []uint32) (*IntervalRelation, error) {
	if left == nil || right == nil {
		return nil, ast.ErrASTDomain
	}
	values, err := joinIntervalColumns(left.values, right.values, leftColumns, rightColumns)
	if err != nil {
		return nil, err
	}
	return &IntervalRelation{values}, nil
}

func (r *IntervalRelation) Project(removed []uint32) (*IntervalRelation, error) {
	if r == nil {
		return nil, ast.ErrASTDomain
	}
	values, err := projectIntervalColumns(r.values, removed)
	if err != nil {
		return nil, err
	}
	if r.Empty() {
		values.setEmpty()
	}
	return &IntervalRelation{values}, nil
}
