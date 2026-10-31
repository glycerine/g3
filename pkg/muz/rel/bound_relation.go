package rel

// Source: src/muz/rel/dl_bound_relation.cpp/.h strict and weak column bounds.

import (
	"sort"

	"github.com/glycerine/g3/pkg/ast"
)

type boundColumn struct{ lt, le map[uint32]bool }

// BoundRelation tracks strict and non-strict order constraints between
// columns. Equality classes and relation-manager registration are pending.
type BoundRelation struct {
	columns []boundColumn
	empty   bool
}

func NewBoundRelation(arity uint32, empty bool) *BoundRelation {
	r := &BoundRelation{columns: make([]boundColumn, int(arity)), empty: empty}
	for i := range r.columns {
		r.columns[i] = boundColumn{lt: make(map[uint32]bool), le: make(map[uint32]bool)}
	}
	return r
}
func (r *BoundRelation) Empty() bool { return r == nil || r.empty }

// AddLess and AddLessEqual propagate the upper column's known bounds.
func (r *BoundRelation) AddLess(left, right uint32) error      { return r.addBound(left, right, true) }
func (r *BoundRelation) AddLessEqual(left, right uint32) error { return r.addBound(left, right, false) }

func (r *BoundRelation) addBound(left, right uint32, strict bool) error {
	if r == nil || uint64(left) >= uint64(len(r.columns)) || uint64(right) >= uint64(len(r.columns)) {
		return ast.ErrASTDomain
	}
	type pending struct {
		column uint32
		strict bool
	}
	todo := []pending{{right, strict}}
	seen := make(map[pending]bool)
	for len(todo) != 0 {
		i := len(todo) - 1
		item := todo[i]
		todo = todo[:i]
		if seen[item] {
			continue
		}
		seen[item] = true
		if item.column == left {
			if item.strict {
				r.empty = true
				return nil
			}
			continue
		}
		source := &r.columns[item.column]
		for next := range source.lt {
			todo = append(todo, pending{next, true})
		}
		for next := range source.le {
			todo = append(todo, pending{next, item.strict})
		}
		if item.strict {
			r.columns[left].lt[item.column] = true
		} else {
			r.columns[left].le[item.column] = true
		}
	}
	return nil
}

// Bounds returns detached, sorted column indices for the source lt/le sets.
func (r *BoundRelation) Bounds(column uint32) (strict, weak []uint32, err error) {
	if r == nil || uint64(column) >= uint64(len(r.columns)) {
		return nil, nil, ast.ErrASTDomain
	}
	for index := range r.columns[column].lt {
		strict = append(strict, index)
	}
	for index := range r.columns[column].le {
		weak = append(weak, index)
	}
	sort.Slice(strict, func(i, j int) bool { return strict[i] < strict[j] })
	sort.Slice(weak, func(i, j int) bool { return weak[i] < weak[j] })
	return strict, weak, nil
}

// UnionInterval drops bounds not established by an interval relation, as in
// bound_relation::mk_union_i. Endpoint openness is ignored by the source.
func (r *BoundRelation) UnionInterval(source *IntervalRelation) error {
	if r == nil || source == nil || source.values == nil || len(r.columns) != len(source.values.columns) {
		return ast.ErrASTDomain
	}
	for i := range r.columns {
		_, _, upper, _ := source.values.columns[source.values.find(uint32(i))].Bounds()
		if upper == nil {
			clear(r.columns[i].lt)
			clear(r.columns[i].le)
			continue
		}
		for j := range r.columns[i].lt {
			lower, _, _, _ := source.values.columns[source.values.find(j)].Bounds()
			if lower == nil || upper.Cmp(lower) >= 0 {
				delete(r.columns[i].lt, j)
			}
		}
		for j := range r.columns[i].le {
			lower, _, _, _ := source.values.columns[source.values.find(j)].Bounds()
			if lower == nil || upper.Cmp(lower) > 0 {
				delete(r.columns[i].le, j)
			}
		}
	}
	return nil
}
