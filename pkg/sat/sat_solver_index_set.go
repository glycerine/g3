package sat

// Source: src/util/hashtable.h:130-627 (core_hashtable) for the index_set of
// the retained SAT consequence algorithm. Iteration is slot order.

import "github.com/glycerine/g3/pkg/arena"

func (s *satSolverIndexSet) open() error {
	if s == nil {
		return ErrSATOwner
	}
	if s.closed {
		return arena.ErrClosed
	}
	return nil
}
func (s *satSolverIndexSet) invalidate() {
	if s.view != nil {
		s.view.valid = false
		s.view = nil
	}
}

// materialize allocates the constructor's initial table.
func (s *satSolverIndexSet) materialize() {
	if s.table == nil {
		s.table = make([]satIndexSlot, satIndexTableInitialCapacity)
	}
}

// satIndexPlace is the probe of copy_table/move_table: the first free slot
// from hash & mask to the end, then from the start.
func satIndexPlace(table []satIndexSlot, key uint32) {
	mask := uint32(len(table) - 1)
	begin := key & mask
	for i := begin; i < uint32(len(table)); i++ {
		if !table[i].used {
			table[i] = satIndexSlot{key: key, used: true}
			return
		}
	}
	for i := uint32(0); i < begin; i++ {
		if !table[i].used {
			table[i] = satIndexSlot{key: key, used: true}
			return
		}
	}
	panic("sat: index_set table full")
}

// satIndexMoveTable is copy_table/move_table: source slots in order, each
// placed in target by probing.
func satIndexMoveTable(source, target []satIndexSlot) {
	for _, slot := range source {
		if slot.used {
			satIndexPlace(target, slot.key)
		}
	}
}

// expandTable is core_hashtable::expand_table.
func (s *satSolverIndexSet) expandTable() {
	table := make([]satIndexSlot, len(s.table)<<1)
	satIndexMoveTable(s.table, table)
	s.table = table
}

// copyFrom is the copy constructor core_hashtable(const core_hashtable&)
// into a fresh set: same capacity, copy_table.
func (s *satSolverIndexSet) copyFrom(other *satSolverIndexSet) {
	capacity := len(other.table)
	if capacity == 0 {
		capacity = satIndexTableInitialCapacity
	}
	s.table = make([]satIndexSlot, capacity)
	satIndexMoveTable(other.table, s.table)
	s.size = other.size
}

func (s *satSolverIndexSet) contains(k uint32) (bool, error) {
	if e := s.open(); e != nil {
		return false, e
	}
	if s.table == nil {
		return false, nil
	}
	mask := uint32(len(s.table) - 1)
	begin := k & mask
	for i := begin; i < uint32(len(s.table)); i++ {
		if !s.table[i].used {
			return false, nil
		}
		if s.table[i].key == k {
			return true, nil
		}
	}
	for i := uint32(0); i < begin; i++ {
		if !s.table[i].used {
			return false, nil
		}
		if s.table[i].key == k {
			return true, nil
		}
	}
	return false, nil
}

// insert is core_hashtable::insert: the growth test runs before the probe,
// so a duplicate insertion can also grow the table.
func (s *satSolverIndexSet) insert(k uint32) error {
	if e := s.open(); e != nil {
		return e
	}
	s.materialize()
	if s.size<<2 > uint32(len(s.table))*3 {
		s.invalidate()
		s.expandTable()
	}
	mask := uint32(len(s.table) - 1)
	begin := k & mask
	place := func(i uint32) bool {
		if s.table[i].used {
			return s.table[i].key == k
		}
		s.invalidate()
		s.table[i] = satIndexSlot{key: k, used: true}
		s.size++
		return true
	}
	for i := begin; i < uint32(len(s.table)); i++ {
		if place(i) {
			return nil
		}
	}
	for i := uint32(0); i < begin; i++ {
		if place(i) {
			return nil
		}
	}
	panic("sat: index_set table full")
}

// reset is core_hashtable::reset: it may halve a mostly free table.
func (s *satSolverIndexSet) reset() error {
	if e := s.open(); e != nil {
		return e
	}
	if s.size == 0 {
		return nil
	}
	s.invalidate()
	overhead := uint32(0)
	for i := range s.table {
		if s.table[i].used {
			s.table[i].used = false
		} else {
			overhead++
		}
	}
	capacity := uint32(len(s.table))
	if capacity > 16 && overhead<<2 > capacity*3 {
		s.table = make([]satIndexSlot, capacity>>1)
	}
	s.size = 0
	return nil
}

// assign is core_hashtable::operator=: reset, then insert other's elements
// in other's slot order.
func (s *satSolverIndexSet) assign(other *satSolverIndexSet) error {
	if e := s.open(); e != nil {
		return e
	}
	if e := other.open(); e != nil {
		return e
	}
	if s == other {
		return nil
	}
	if e := s.reset(); e != nil {
		return e
	}
	return s.insertAll(other)
}

// union is core_hashtable::operator|=.
func (s *satSolverIndexSet) union(other *satSolverIndexSet) error {
	if e := s.open(); e != nil {
		return e
	}
	if e := other.open(); e != nil {
		return e
	}
	if s == other {
		return nil
	}
	return s.insertAll(other)
}

func (s *satSolverIndexSet) insertAll(other *satSolverIndexSet) error {
	for _, k := range other.slotKeys() {
		if e := s.insert(k); e != nil {
			return e
		}
	}
	return nil
}

// slotKeys lists the elements in iterator (slot) order.
func (s *satSolverIndexSet) slotKeys() []uint32 {
	keys := make([]uint32, 0, s.size)
	for _, slot := range s.table {
		if slot.used {
			keys = append(keys, slot.key)
		}
	}
	return keys
}

func (s *satSolverIndexSet) close() error {
	if s == nil {
		return ErrSATOwner
	}
	if s.closed {
		return nil
	}
	s.invalidate()
	s.table = nil
	s.size = 0
	s.closed = true
	return nil
}

type satSolverIndexValue struct {
	value   uint32
	present bool
}
type satSolverIndexView struct {
	keys  []uint32
	valid bool
}
type satSolverIndexIterator struct {
	view  *satSolverIndexView
	index int
}

// Iterators borrow one slot-ordered view until the table changes, reset or
// close occurs. Duplicate insertions that do not grow the table leave that
// view valid.
func (s *satSolverIndexSet) capture() (satSolverIndexIterator, error) {
	if e := s.open(); e != nil {
		return satSolverIndexIterator{}, e
	}
	if s.view == nil {
		s.view = &satSolverIndexView{valid: true, keys: s.slotKeys()}
	}
	return satSolverIndexIterator{view: s.view}, nil
}
func (it *satSolverIndexIterator) next() (satSolverIndexValue, error) {
	if it == nil || it.view == nil || !it.view.valid {
		return satSolverIndexValue{}, ErrSATOwner
	}
	if it.index == len(it.view.keys) {
		return satSolverIndexValue{}, nil
	}
	v := it.view.keys[it.index]
	it.index++
	return satSolverIndexValue{v, true}, nil
}
