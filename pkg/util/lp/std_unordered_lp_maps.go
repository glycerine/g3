package lp

// Source: libstdc++ (GCC 13) std::unordered_map node order, as used by
// src/util/lp/lp_solver.h (m_A_values) and src/util/lp/mps_reader.h
// (m_rows, row::m_row_columns, m_columns).

// =========================================================================
// !!! WARNING: THIS IS A DELIBERATE, USER-APPROVED PORT OF STL INTERNALS !!!
// =========================================================================
//
// Approved by the owner on 2026-11-14 as a generalization of the lar_term
// (StdUnorderedRationalMap) and m_names_to_columns (StdUnorderedStringMap)
// exceptions. DO NOT replace these types with Go maps, sorted slices or
// insdict.
//
// WHY THEY EXIST:
//   mps_reader::fill_solver_on_rows walks m_rows and each row's
//   m_row_columns, which fixes the insertion order of lp_solver's
//   m_A_values and of every row's column map; lp_solver then numbers the
//   core rows and columns by walking m_A_values
//   (map_external_rows_to_core_solver_rows,
//   map_external_columns_to_core_solver_columns), and sums floating values
//   in those orders. All of them are libstdc++ node orders, so the core
//   problem z3 solves (and with a degenerate optimum, the vertex it
//   reports) depends on them.
//
// The node placement is the shared stdHashtable of std_unordered_map.go;
// the keyed slot maps below (StdUnorderedUintMap for unsigned keys,
// StdUnorderedStringMap for std::string keys) are pinned against g++ 13 by
// TestStdUnorderedMapMatchesLibstdcxx and
// TestStdUnorderedStringMapMatchesLibstdcxx. The value wrappers only hold a
// value per slot; they never reorder.

import "github.com/glycerine/g3/pkg/util/numeral"

// StdUnorderedUintMap is std::unordered_map<unsigned, unsigned>: iteration
// follows libstdc++'s node list. The Go map only speeds up key lookup.
type StdUnorderedUintMap struct {
	table  stdHashtable
	keys   []uint32
	values []uint32
	lookup map[uint32]int32
}

// NewStdUnorderedUintMap is a default-constructed map (one bucket).
func NewStdUnorderedUintMap() *StdUnorderedUintMap {
	return &StdUnorderedUintMap{table: newStdHashtable(), lookup: map[uint32]int32{}}
}

// Len is size().
func (m *StdUnorderedUintMap) Len() int { return int(m.table.count) }

// BucketCount is bucket_count().
func (m *StdUnorderedUintMap) BucketCount() uint64 { return uint64(len(m.table.buckets)) }

// Get2 is find(key).
func (m *StdUnorderedUintMap) Get2(key uint32) (uint32, bool) {
	n, ok := m.lookup[key]
	if !ok {
		return 0, false
	}
	return m.values[n], true
}

// Put is m[key] = value: a new key is inserted as _M_insert_unique_node
// does (std::hash<unsigned> is the identity), an existing key keeps its
// node.
func (m *StdUnorderedUintMap) Put(key uint32, value uint32) {
	if n, ok := m.lookup[key]; ok {
		m.values[n] = value
		return
	}
	n, appended := m.table.allocNode(uint64(key))
	if appended {
		m.keys = append(m.keys, key)
		m.values = append(m.values, value)
	} else {
		m.keys[n], m.values[n] = key, value
	}
	m.lookup[key] = n
	m.table.linkNode(n)
}

// Del is erase(const key_type&).
func (m *StdUnorderedUintMap) Del(key uint32) {
	n, ok := m.lookup[key]
	if !ok {
		return
	}
	m.table.unlinkNode(n)
	delete(m.lookup, key)
}

// Clear is clear(): the bucket array and rehash state stay.
func (m *StdUnorderedUintMap) Clear() {
	m.table.clear()
	m.keys, m.values = m.keys[:0], m.values[:0]
	clear(m.lookup)
}

// All iterates in begin()..end() order. The yielded entry may be deleted
// during iteration; other mutations invalidate the walk.
func (m *StdUnorderedUintMap) All() func(yield func(uint32, uint32) bool) {
	return func(yield func(uint32, uint32) bool) {
		for p := m.table.beforeBegin; p != stdNil; {
			next := m.table.next[p]
			if !yield(m.keys[p], m.values[p]) {
				return
			}
			p = next
		}
	}
}

// Keys returns the keys in iteration order.
func (m *StdUnorderedUintMap) Keys() []uint32 {
	out := make([]uint32, 0, m.table.count)
	for p := m.table.beforeBegin; p != stdNil; p = m.table.next[p] {
		out = append(out, m.keys[p])
	}
	return out
}

// StdUnorderedFloatRow is std::unordered_map<unsigned, double>, one row of
// lp_solver<double, double>::m_A_values.
type StdUnorderedFloatRow struct {
	slots  *StdUnorderedUintMap
	values []float64
}

func NewStdUnorderedFloatRow() *StdUnorderedFloatRow {
	return &StdUnorderedFloatRow{slots: NewStdUnorderedUintMap()}
}

func (r *StdUnorderedFloatRow) Len() int { return r.slots.Len() }

func (r *StdUnorderedFloatRow) Get2(j uint32) (float64, bool) {
	slot, ok := r.slots.Get2(j)
	if !ok {
		return 0, false
	}
	return r.values[slot], true
}

// Get is find(j)->second, or 0 when j is absent.
func (r *StdUnorderedFloatRow) Get(j uint32) float64 {
	v, _ := r.Get2(j)
	return v
}

// Set is row[j] = v.
func (r *StdUnorderedFloatRow) Set(j uint32, v float64) {
	if slot, ok := r.slots.Get2(j); ok {
		r.values[slot] = v
		return
	}
	r.slots.Put(j, uint32(len(r.values)))
	r.values = append(r.values, v)
}

// Del is erase(j).
func (r *StdUnorderedFloatRow) Del(j uint32) { r.slots.Del(j) }

// All iterates in libstdc++ node order.
func (r *StdUnorderedFloatRow) All() func(yield func(uint32, float64) bool) {
	return func(yield func(uint32, float64) bool) {
		for j, slot := range r.slots.All() {
			if !yield(j, r.values[slot]) {
				return
			}
		}
	}
}

// Keys returns the column keys in iteration order.
func (r *StdUnorderedFloatRow) Keys() []uint32 { return r.slots.Keys() }

// StdUnorderedFloatRows is lp_solver<double, double>::m_A_values,
// std::unordered_map<unsigned, std::unordered_map<unsigned, double>>.
type StdUnorderedFloatRows struct {
	slots *StdUnorderedUintMap
	rows  []*StdUnorderedFloatRow
}

func NewStdUnorderedFloatRows() *StdUnorderedFloatRows {
	return &StdUnorderedFloatRows{slots: NewStdUnorderedUintMap()}
}

func (m *StdUnorderedFloatRows) Len() int { return m.slots.Len() }

func (m *StdUnorderedFloatRows) Get2(i uint32) (*StdUnorderedFloatRow, bool) {
	slot, ok := m.slots.Get2(i)
	if !ok {
		return nil, false
	}
	return m.rows[slot], true
}

// Set is m_A_values[i] = row.
func (m *StdUnorderedFloatRows) Set(i uint32, row *StdUnorderedFloatRow) {
	if slot, ok := m.slots.Get2(i); ok {
		m.rows[slot] = row
		return
	}
	m.slots.Put(i, uint32(len(m.rows)))
	m.rows = append(m.rows, row)
}

// Del is erase(i).
func (m *StdUnorderedFloatRows) Del(i uint32) { m.slots.Del(i) }

// All iterates in libstdc++ node order.
func (m *StdUnorderedFloatRows) All() func(yield func(uint32, *StdUnorderedFloatRow) bool) {
	return func(yield func(uint32, *StdUnorderedFloatRow) bool) {
		for i, slot := range m.slots.All() {
			if !yield(i, m.rows[slot]) {
				return
			}
		}
	}
}

// Keys returns the row keys in iteration order.
func (m *StdUnorderedFloatRows) Keys() []uint32 { return m.slots.Keys() }

// StdUnorderedRationalRows is lp_solver<mpq, mpq>::m_A_values,
// std::unordered_map<unsigned, std::unordered_map<unsigned, mpq>>.
type StdUnorderedRationalRows struct {
	slots *StdUnorderedUintMap
	rows  []*StdUnorderedRationalMap
}

func NewStdUnorderedRationalRows() *StdUnorderedRationalRows {
	return &StdUnorderedRationalRows{slots: NewStdUnorderedUintMap()}
}

func (m *StdUnorderedRationalRows) Len() int { return m.slots.Len() }

func (m *StdUnorderedRationalRows) Get2(i uint32) (*StdUnorderedRationalMap, bool) {
	slot, ok := m.slots.Get2(i)
	if !ok {
		return nil, false
	}
	return m.rows[slot], true
}

// Set is m_A_values[i] = row.
func (m *StdUnorderedRationalRows) Set(i uint32, row *StdUnorderedRationalMap) {
	if slot, ok := m.slots.Get2(i); ok {
		m.rows[slot] = row
		return
	}
	m.slots.Put(i, uint32(len(m.rows)))
	m.rows = append(m.rows, row)
}

// Del is erase(i).
func (m *StdUnorderedRationalRows) Del(i uint32) { m.slots.Del(i) }

// All iterates in libstdc++ node order.
func (m *StdUnorderedRationalRows) All() func(yield func(uint32, *StdUnorderedRationalMap) bool) {
	return func(yield func(uint32, *StdUnorderedRationalMap) bool) {
		for i, slot := range m.slots.All() {
			if !yield(i, m.rows[slot]) {
				return
			}
		}
	}
}

// Keys returns the row keys in iteration order.
func (m *StdUnorderedRationalRows) Keys() []uint32 { return m.slots.Keys() }

// StdUnorderedStringFloatMap is std::unordered_map<std::string, double>,
// mps_reader<double, double>::row::m_row_columns.
type StdUnorderedStringFloatMap struct {
	slots  *StdUnorderedStringMap
	values []float64
}

func NewStdUnorderedStringFloatMap() *StdUnorderedStringFloatMap {
	return &StdUnorderedStringFloatMap{slots: NewStdUnorderedStringMap()}
}

func (m *StdUnorderedStringFloatMap) Len() int { return m.slots.Len() }

func (m *StdUnorderedStringFloatMap) Get2(name string) (float64, bool) {
	slot, ok := m.slots.Get2(name)
	if !ok {
		return 0, false
	}
	return m.values[slot], true
}

// Get is find(name)->second, or 0 when name is absent.
func (m *StdUnorderedStringFloatMap) Get(name string) float64 {
	v, _ := m.Get2(name)
	return v
}

// Set is m[name] = v.
func (m *StdUnorderedStringFloatMap) Set(name string, v float64) {
	if slot, ok := m.slots.Get2(name); ok {
		m.values[slot] = v
		return
	}
	m.slots.Put(name, uint32(len(m.values)))
	m.values = append(m.values, v)
}

// All iterates in libstdc++ node order.
func (m *StdUnorderedStringFloatMap) All() func(yield func(string, float64) bool) {
	return func(yield func(string, float64) bool) {
		for name, slot := range m.slots.All() {
			if !yield(name, m.values[slot]) {
				return
			}
		}
	}
}

// Keys returns the names in iteration order.
func (m *StdUnorderedStringFloatMap) Keys() []string { return m.slots.Keys() }

// StdUnorderedStringRationalMap is std::unordered_map<std::string, mpq>,
// mps_reader<mpq, mpq>::row::m_row_columns.
type StdUnorderedStringRationalMap struct {
	slots  *StdUnorderedStringMap
	values []*numeral.Rational
}

func NewStdUnorderedStringRationalMap() *StdUnorderedStringRationalMap {
	return &StdUnorderedStringRationalMap{slots: NewStdUnorderedStringMap()}
}

func (m *StdUnorderedStringRationalMap) Len() int { return m.slots.Len() }

func (m *StdUnorderedStringRationalMap) Get2(name string) (*numeral.Rational, bool) {
	slot, ok := m.slots.Get2(name)
	if !ok {
		return nil, false
	}
	return m.values[slot], true
}

// Get is find(name)->second, or nil when name is absent.
func (m *StdUnorderedStringRationalMap) Get(name string) *numeral.Rational {
	v, _ := m.Get2(name)
	return v
}

// Set is m[name] = v.
func (m *StdUnorderedStringRationalMap) Set(name string, v *numeral.Rational) {
	if slot, ok := m.slots.Get2(name); ok {
		m.values[slot] = v
		return
	}
	m.slots.Put(name, uint32(len(m.values)))
	m.values = append(m.values, v)
}

// All iterates in libstdc++ node order.
func (m *StdUnorderedStringRationalMap) All() func(yield func(string, *numeral.Rational) bool) {
	return func(yield func(string, *numeral.Rational) bool) {
		for name, slot := range m.slots.All() {
			if !yield(name, m.values[slot]) {
				return
			}
		}
	}
}

// Keys returns the names in iteration order.
func (m *StdUnorderedStringRationalMap) Keys() []string { return m.slots.Keys() }

// Set is m[key] = value (Put), the spelling the lp_solver row maps use.
func (m *StdUnorderedRationalMap) Set(key uint32, value *numeral.Rational) { m.Put(key, value) }

// StdUnorderedRationalColumns is lp_solver<mpq, mpq>::
// m_map_from_var_index_to_column_info,
// std::unordered_map<unsigned, column_info<mpq> *>.
type StdUnorderedRationalColumns struct {
	slots *StdUnorderedUintMap
	cols  []*RationalColumnInfo
}

func NewStdUnorderedRationalColumns() *StdUnorderedRationalColumns {
	return &StdUnorderedRationalColumns{slots: NewStdUnorderedUintMap()}
}

func (m *StdUnorderedRationalColumns) Len() int { return m.slots.Len() }

func (m *StdUnorderedRationalColumns) Get2(j uint32) (*RationalColumnInfo, bool) {
	slot, ok := m.slots.Get2(j)
	if !ok {
		return nil, false
	}
	return m.cols[slot], true
}

// Get is find(j)->second, or nil when j is absent.
func (m *StdUnorderedRationalColumns) Get(j uint32) *RationalColumnInfo {
	c, _ := m.Get2(j)
	return c
}

// Set is m_map_from_var_index_to_column_info[j] = c.
func (m *StdUnorderedRationalColumns) Set(j uint32, c *RationalColumnInfo) {
	if slot, ok := m.slots.Get2(j); ok {
		m.cols[slot] = c
		return
	}
	m.slots.Put(j, uint32(len(m.cols)))
	m.cols = append(m.cols, c)
}

// Del is erase(j).
func (m *StdUnorderedRationalColumns) Del(j uint32) { m.slots.Del(j) }

// All iterates in libstdc++ node order.
func (m *StdUnorderedRationalColumns) All() func(yield func(uint32, *RationalColumnInfo) bool) {
	return func(yield func(uint32, *RationalColumnInfo) bool) {
		for j, slot := range m.slots.All() {
			if !yield(j, m.cols[slot]) {
				return
			}
		}
	}
}

// Keys returns the keys in iteration order.
func (m *StdUnorderedRationalColumns) Keys() []uint32 { return m.slots.Keys() }
