package lp

// Source: src/util/lp/mps_reader.h:156
// (std::unordered_map<std::string, column *> m_columns).

// floatMPSColumnMap is mps_reader's m_columns: its iteration order is the
// libstdc++ node order (StdUnorderedStringMap, user-approved for AUDIT.md
// util_lp_06-F1), because fill_solver_on_columns (mps_reader.h:715) walks
// it to call give_symbolic_name_to_column, which fixes the order of
// lp_solver::print_model. The string map holds a slot into cols.
type floatMPSColumnMap struct {
	slots *StdUnorderedStringMap
	cols  []*floatMPSColumn
}

func newFloatMPSColumnMap() *floatMPSColumnMap {
	return &floatMPSColumnMap{slots: NewStdUnorderedStringMap()}
}

// Len is size().
func (m *floatMPSColumnMap) Len() int { return m.slots.Len() }

// Get2 is find(name).
func (m *floatMPSColumnMap) Get2(name string) (*floatMPSColumn, bool) {
	slot, ok := m.slots.Get2(name)
	if !ok {
		return nil, false
	}
	return m.cols[slot], true
}

// Set is m_columns[name] = col.
func (m *floatMPSColumnMap) Set(name string, col *floatMPSColumn) {
	if slot, ok := m.slots.Get2(name); ok {
		m.cols[slot] = col
		return
	}
	m.slots.Put(name, uint32(len(m.cols)))
	m.cols = append(m.cols, col)
}

// All iterates in begin()..end() order.
func (m *floatMPSColumnMap) All() func(yield func(string, *floatMPSColumn) bool) {
	return func(yield func(string, *floatMPSColumn) bool) {
		for name, slot := range m.slots.All() {
			if !yield(name, m.cols[slot]) {
				return
			}
		}
	}
}

// floatMPSRowMap is mps_reader's std::unordered_map<std::string, row *>
// m_rows: fill_solver_on_rows (mps_reader.h:708) walks it in libstdc++
// node order, which fixes the insertion order of lp_solver's m_A_values
// (std_unordered_lp_maps.go). The string map holds a slot into rows.
type floatMPSRowMap struct {
	slots *StdUnorderedStringMap
	rows  []*floatMPSRow
}

func newFloatMPSRowMap() *floatMPSRowMap {
	return &floatMPSRowMap{slots: NewStdUnorderedStringMap()}
}

// Len is size().
func (m *floatMPSRowMap) Len() int { return m.slots.Len() }

// Get2 is find(name).
func (m *floatMPSRowMap) Get2(name string) (*floatMPSRow, bool) {
	slot, ok := m.slots.Get2(name)
	if !ok {
		return nil, false
	}
	return m.rows[slot], true
}

// Set is m_rows[name] = row.
func (m *floatMPSRowMap) Set(name string, row *floatMPSRow) {
	if slot, ok := m.slots.Get2(name); ok {
		m.rows[slot] = row
		return
	}
	m.slots.Put(name, uint32(len(m.rows)))
	m.rows = append(m.rows, row)
}

// All iterates in begin()..end() order.
func (m *floatMPSRowMap) All() func(yield func(string, *floatMPSRow) bool) {
	return func(yield func(string, *floatMPSRow) bool) {
		for name, slot := range m.slots.All() {
			if !yield(name, m.rows[slot]) {
				return
			}
		}
	}
}
