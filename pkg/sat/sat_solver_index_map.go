package sat

// Source: src/util/map.h (table2map) over src/util/hashtable.h
// (core_hashtable) for u_map<index_set> m_antecedents.

import "github.com/glycerine/g3/pkg/arena"

func (m *satSolverIndexMap) open() error {
	if m == nil || m.self != m {
		return ErrSATOwner
	}
	if m.closed {
		return arena.ErrClosed
	}
	return nil
}

// satIndexMapDeleteTable is delete_table: every entry's value is destroyed,
// so references into the old table go stale.
func satIndexMapDeleteTable(table []satIndexMapEntry) {
	for i := range table {
		table[i].value.close()
	}
}

// findCore is core_hashtable::find_core.
func (m *satSolverIndexMap) findCore(k uint32) *satIndexMapEntry {
	mask := uint32(len(m.table) - 1)
	begin := k & mask
	for i := begin; i < uint32(len(m.table)); i++ {
		if !m.table[i].used {
			return nil
		}
		if m.table[i].key == k {
			return &m.table[i]
		}
	}
	for i := uint32(0); i < begin; i++ {
		if !m.table[i].used {
			return nil
		}
		if m.table[i].key == k {
			return &m.table[i]
		}
	}
	return nil
}

func (m *satSolverIndexMap) contains(k uint32) (bool, error) {
	if e := m.open(); e != nil {
		return false, e
	}
	return m.findCore(k) != nil, nil
}

// lookup is find_core's value, or nil when k is absent.
func (m *satSolverIndexMap) lookup(k uint32) *satSolverIndexSet {
	if entry := m.findCore(k); entry != nil {
		return &entry.value
	}
	return nil
}

// find is table2map::find(k): a reference to the stored value.
func (m *satSolverIndexMap) find(k uint32) (*satSolverIndexSet, error) {
	if e := m.open(); e != nil {
		return nil, e
	}
	if row := m.lookup(k); row != nil {
		return row, nil
	}
	if satStorageSourceDebug {
		return nil, ErrSATAssertion
	}
	return nil, arena.ErrBounds
}

// expandTable is core_hashtable::expand_table: move_table assigns each used
// entry, in slot order, into a default-constructed entry of the new table;
// key_data's move assignment is index_set's copy assignment.
func (m *satSolverIndexMap) expandTable() error {
	table := make([]satIndexMapEntry, len(m.table)<<1)
	mask := uint32(len(table) - 1)
	for i := range m.table {
		source := &m.table[i]
		if !source.used {
			continue
		}
		begin := source.key & mask
		target := -1
		for j := begin; j < uint32(len(table)); j++ {
			if !table[j].used {
				target = int(j)
				break
			}
		}
		for j := uint32(0); target < 0 && j < begin; j++ {
			if !table[j].used {
				target = int(j)
			}
		}
		if target < 0 {
			panic("sat: u_map table full")
		}
		table[target].key, table[target].used = source.key, true
		if e := table[target].value.assign(&source.value); e != nil {
			return e
		}
	}
	satIndexMapDeleteTable(m.table)
	m.table = table
	return nil
}

// insert is table2map::insert(k, v): m_table.insert(key_data(k, v)). The
// temporary key_data copy-constructs v; storing it into the (possibly
// reused, free) entry is index_set's copy assignment from the temporary.
func (m *satSolverIndexMap) insert(k uint32, value *satSolverIndexSet) error {
	if e := m.open(); e != nil {
		return e
	}
	if e := value.open(); e != nil {
		return e
	}
	var tmp satSolverIndexSet
	tmp.copyFrom(value)
	defer tmp.close()
	if m.size<<2 > uint32(len(m.table))*3 {
		if e := m.expandTable(); e != nil {
			return e
		}
	}
	mask := uint32(len(m.table) - 1)
	begin := k & mask
	store := func(i uint32) (bool, error) {
		entry := &m.table[i]
		if entry.used {
			if entry.key != k {
				return false, nil
			}
			return true, entry.value.assign(&tmp)
		}
		entry.key, entry.used = k, true
		m.size++
		return true, entry.value.assign(&tmp)
	}
	for i := begin; i < uint32(len(m.table)); i++ {
		if done, e := store(i); done || e != nil {
			return e
		}
	}
	for i := uint32(0); i < begin; i++ {
		if done, e := store(i); done || e != nil {
			return e
		}
	}
	panic("sat: u_map table full")
}

// reset is core_hashtable::reset: entries are marked free and keep their
// values; a mostly free table larger than 16 is halved.
func (m *satSolverIndexMap) reset() error {
	if e := m.open(); e != nil {
		return e
	}
	if m.size == 0 {
		return nil
	}
	overhead := uint32(0)
	for i := range m.table {
		if m.table[i].used {
			m.table[i].used = false
		} else {
			overhead++
		}
	}
	capacity := uint32(len(m.table))
	if capacity > 16 && overhead<<2 > capacity*3 {
		satIndexMapDeleteTable(m.table)
		m.table = make([]satIndexMapEntry, capacity>>1)
	}
	m.size = 0
	return nil
}
