package lp

// Source: src/util/lp/mps_reader.h:155-156 (std::unordered_map<std::string,
// row *> m_rows, std::unordered_map<std::string, column *> m_columns) for
// mps_reader<mpq, mpq>. Both iterate in libstdc++ node order (the
// owner-approved emulation of std_unordered_lp_maps.go); the string maps
// hold slots into the value slices.

type rationalMPSRowMap struct {
	slots *StdUnorderedStringMap
	rows  []*rationalMPSRow
}

func newRationalMPSRowMap() *rationalMPSRowMap {
	return &rationalMPSRowMap{slots: NewStdUnorderedStringMap()}
}

// Len is size().
func (m *rationalMPSRowMap) Len() int { return m.slots.Len() }

// Get2 is find(name).
func (m *rationalMPSRowMap) Get2(name string) (*rationalMPSRow, bool) {
	slot, ok := m.slots.Get2(name)
	if !ok {
		return nil, false
	}
	return m.rows[slot], true
}

// Set is m_rows[name] = row.
func (m *rationalMPSRowMap) Set(name string, row *rationalMPSRow) {
	if slot, ok := m.slots.Get2(name); ok {
		m.rows[slot] = row
		return
	}
	m.slots.Put(name, uint32(len(m.rows)))
	m.rows = append(m.rows, row)
}

// All iterates in begin()..end() order.
func (m *rationalMPSRowMap) All() func(yield func(string, *rationalMPSRow) bool) {
	return func(yield func(string, *rationalMPSRow) bool) {
		for name, slot := range m.slots.All() {
			if !yield(name, m.rows[slot]) {
				return
			}
		}
	}
}

type rationalMPSColumnMap struct {
	slots *StdUnorderedStringMap
	cols  []*rationalMPSColumn
}

func newRationalMPSColumnMap() *rationalMPSColumnMap {
	return &rationalMPSColumnMap{slots: NewStdUnorderedStringMap()}
}

// Len is size().
func (m *rationalMPSColumnMap) Len() int { return m.slots.Len() }

// Get2 is find(name).
func (m *rationalMPSColumnMap) Get2(name string) (*rationalMPSColumn, bool) {
	slot, ok := m.slots.Get2(name)
	if !ok {
		return nil, false
	}
	return m.cols[slot], true
}

// Set is m_columns[name] = col.
func (m *rationalMPSColumnMap) Set(name string, col *rationalMPSColumn) {
	if slot, ok := m.slots.Get2(name); ok {
		m.cols[slot] = col
		return
	}
	m.slots.Put(name, uint32(len(m.cols)))
	m.cols = append(m.cols, col)
}

// All iterates in begin()..end() order.
func (m *rationalMPSColumnMap) All() func(yield func(string, *rationalMPSColumn) bool) {
	return func(yield func(string, *rationalMPSColumn) bool) {
		for name, slot := range m.slots.All() {
			if !yield(name, m.cols[slot]) {
				return
			}
		}
	}
}
