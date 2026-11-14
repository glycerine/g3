package sat

// Source: src/sat/sat_solver.h:481-483 (index_set, u_map<index_set>
// m_antecedents); src/util/hashtable.h (core_hashtable) and src/util/map.h
// (table2map). The concrete open-addressing tables are kept because the
// consequence algorithm emits index_set elements in slot order.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

// satIndexTableInitialCapacity is DEFAULT_HASHTABLE_INITIAL_CAPACITY.
const satIndexTableInitialCapacity = 8

// satIndexSlot is default_hash_entry<unsigned> with u_hash (identity): no
// index_set element is ever removed, so there is no deleted state.
type satIndexSlot struct {
	key  uint32
	used bool
}

// satSolverIndexSet is hashtable<unsigned, u_hash, u_eq>. A nil table is a
// freshly constructed set of capacity satIndexTableInitialCapacity.
type satSolverIndexSet struct {
	table  []satIndexSlot
	size   uint32
	view   *satSolverIndexView
	closed bool
}

// satIndexMapEntry is default_map_entry<unsigned, index_set>: marking an
// entry free keeps its value, as the source does.
type satIndexMapEntry struct {
	key   uint32
	used  bool
	value satSolverIndexSet
}

// satSolverIndexMap is u_map<index_set> (table2map over core_hashtable).
type satSolverIndexMap struct {
	self       *satSolverIndexMap
	budget     *arena.Budget
	table      []satIndexMapEntry
	size       uint32
	ownerBytes uint64
	closed     bool
}

func newSATSolverIndexMap(b *arena.Budget) (*satSolverIndexMap, error) {
	bytes := uint64(reflect.TypeOf(satSolverIndexMap{}).Size())
	if e := b.Reserve(bytes); e != nil {
		return nil, e
	}
	m := &satSolverIndexMap{budget: b, ownerBytes: bytes, table: make([]satIndexMapEntry, satIndexTableInitialCapacity)}
	m.self = m
	return m, nil
}
func (m *satSolverIndexMap) Close() error {
	if m == nil || m.self != m {
		return ErrSATOwner
	}
	if m.closed {
		return nil
	}
	satIndexMapDeleteTable(m.table)
	m.table = nil
	m.size = 0
	m.budget.Release(m.ownerBytes)
	m.ownerBytes = 0
	m.closed = true
	return nil
}
