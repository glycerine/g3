package simplex

// Package simplex implements the retained exact sparse tableau and simplex
// engines from src/math/simplex.

import (
	"fmt"
	"io"
	"math/big"
)

const NullVariable = ^uint32(0)

// Row is a stable matrix row identifier. Deleted identifiers are recycled by
// the next NewRow call, as in sparse_matrix::mk_row.
type Row struct{ id uint32 }

func (r Row) ID() uint32 { return r.id }

// RowEntry is a borrowed coefficient-variable pair returned by Entries.
type RowEntry struct {
	Variable    uint32
	Coefficient *big.Rat
}

// matrixEntry is sparse_matrix::_row_entry. A dead entry is on its row's
// LIFO free list (src/math/simplex/sparse_matrix_def.h:57-86).
type matrixEntry struct {
	variable uint32
	coeff    big.Rat
	colIdx   int
	dead     bool
	nextFree int
}
type matrixRow struct {
	entries   []matrixEntry
	size      int
	firstFree int
	live      bool
}

// matrixColEntry is sparse_matrix::col_entry.
type matrixColEntry struct {
	row      uint32
	rowIdx   int
	dead     bool
	nextFree int
}

// matrixColumn is sparse_matrix::column. refs counts outstanding column
// iterations (m_refs); a referenced column is never compressed.
type matrixColumn struct {
	entries   []matrixColEntry
	size      int
	firstFree int
	refs      int
}

// Matrix is a concrete exact-rational replacement for the two retained C++
// sparse_matrix instantiations. Integer coefficients use denominator one.
// Rows and columns keep the source slot layout: deleted entries are reused
// LIFO by later insertions and compacted only when fewer than half of the
// slots are live, so iteration follows the source slot order.
type Matrix struct {
	rows    []matrixRow
	dead    []uint32
	columns []matrixColumn
	addRows uint32
}

func (m *Matrix) EnsureVariable(v uint32) {
	for uint32(len(m.columns)) <= v {
		m.columns = append(m.columns, matrixColumn{firstFree: -1})
	}
}

// NewRow is sparse_matrix::mk_row. A recycled row keeps its dead slots and
// free list, as the source does not reset it.
func (m *Matrix) NewRow() Row {
	if len(m.dead) != 0 {
		i := m.dead[len(m.dead)-1]
		m.dead = m.dead[:len(m.dead)-1]
		m.rows[i].live = true
		return Row{i}
	}
	r := Row{uint32(len(m.rows))}
	m.rows = append(m.rows, matrixRow{firstFree: -1, live: true})
	return r
}

func (m *Matrix) valid(r Row) bool { return int(r.id) < len(m.rows) && m.rows[r.id].live }

// addRowEntry is _row::add_row_entry.
func (row *matrixRow) addRowEntry() int {
	row.size++
	if row.firstFree == -1 {
		row.entries = append(row.entries, matrixEntry{})
		return len(row.entries) - 1
	}
	idx := row.firstFree
	row.firstFree = row.entries[idx].nextFree
	row.entries[idx].dead = false
	return idx
}

// delRowEntry is _row::del_row_entry.
func (row *matrixRow) delRowEntry(idx int) {
	e := &row.entries[idx]
	e.nextFree = row.firstFree
	e.dead = true
	e.coeff.SetInt64(0)
	row.size--
	row.firstFree = idx
}

// addColEntry is column::add_col_entry.
func (c *matrixColumn) addColEntry() int {
	c.size++
	if c.firstFree == -1 {
		c.entries = append(c.entries, matrixColEntry{})
		return len(c.entries) - 1
	}
	idx := c.firstFree
	c.firstFree = c.entries[idx].nextFree
	c.entries[idx].dead = false
	return idx
}

// delColEntry is column::del_col_entry.
func (c *matrixColumn) delColEntry(idx int) {
	e := &c.entries[idx]
	e.dead = true
	e.nextFree = c.firstFree
	c.firstFree = idx
	c.size--
}

// compressRow is _row::compress.
func (m *Matrix) compressRow(r uint32) {
	row := &m.rows[r]
	j := 0
	for i := range row.entries {
		if row.entries[i].dead {
			continue
		}
		if i != j {
			row.entries[j].variable = row.entries[i].variable
			row.entries[j].coeff.Set(&row.entries[i].coeff)
			row.entries[j].colIdx = row.entries[i].colIdx
			row.entries[j].dead = false
			col := &m.columns[row.entries[j].variable]
			col.entries[row.entries[j].colIdx].rowIdx = j
		}
		j++
	}
	row.entries = row.entries[:row.size]
	row.firstFree = -1
}

// compressColumn is column::compress.
func (m *Matrix) compressColumn(v uint32) {
	c := &m.columns[v]
	j := 0
	for i := range c.entries {
		if c.entries[i].dead {
			continue
		}
		if i != j {
			c.entries[j] = c.entries[i]
			e := &c.entries[j]
			m.rows[e.row].entries[e.rowIdx].colIdx = j
		}
		j++
	}
	c.entries = c.entries[:c.size]
	c.firstFree = -1
}

// compressColumnIfNeeded is column::compress_if_needed.
func (m *Matrix) compressColumnIfNeeded(v uint32) {
	c := &m.columns[v]
	if c.size*2 < len(c.entries) && c.refs == 0 {
		m.compressColumn(v)
	}
}

// RetainColumn and ReleaseColumn bracket a source col_iterator's lifetime
// (m_refs), which suppresses compression of the iterated column.
func (m *Matrix) RetainColumn(v uint32) {
	m.EnsureVariable(v)
	m.columns[v].refs++
}
func (m *Matrix) ReleaseColumn(v uint32) {
	if int(v) < len(m.columns) && m.columns[v].refs > 0 {
		m.columns[v].refs--
	}
}

// addVar is sparse_matrix::add_var.
func (m *Matrix) addVar(r uint32, coefficient *big.Rat, variable uint32) {
	row := &m.rows[r]
	rIdx := row.addRowEntry()
	col := &m.columns[variable]
	cIdx := col.addColEntry()
	e := &row.entries[rIdx]
	e.variable = variable
	e.coeff.Set(coefficient)
	e.colIdx = cIdx
	col.entries[cIdx].row = r
	col.entries[cIdx].rowIdx = rIdx
}

// deleteRowEntry is sparse_matrix::del_row_entry.
func (m *Matrix) deleteRowEntry(r uint32, pos int) {
	row := &m.rows[r]
	v := row.entries[pos].variable
	colIdx := row.entries[pos].colIdx
	row.delRowEntry(pos)
	m.columns[v].delColEntry(colIdx)
	m.compressColumnIfNeeded(v)
}

func (m *Matrix) entryIndex(r Row, variable uint32) int {
	row := &m.rows[r.id]
	for i := range row.entries {
		if !row.entries[i].dead && row.entries[i].variable == variable {
			return i
		}
	}
	return -1
}

func (m *Matrix) AddRational(r Row, coefficient *big.Rat, variable uint32) error {
	if !m.valid(r) || coefficient == nil || coefficient.Sign() == 0 {
		return fmt.Errorf("simplex: invalid matrix insertion")
	}
	m.EnsureVariable(variable)
	if m.entryIndex(r, variable) >= 0 {
		return fmt.Errorf("simplex: duplicate variable %d", variable)
	}
	m.addVar(r.id, coefficient, variable)
	return nil
}

func (m *Matrix) AddInteger(r Row, coefficient *big.Int, variable uint32) error {
	if coefficient == nil {
		return fmt.Errorf("simplex: nil coefficient")
	}
	return m.AddRational(r, new(big.Rat).SetInt(coefficient), variable)
}

// Entries lists the live entries in slot order (row_iterator).
func (m *Matrix) Entries(r Row) ([]RowEntry, error) {
	if !m.valid(r) {
		return nil, fmt.Errorf("simplex: invalid row")
	}
	row := &m.rows[r.id]
	out := make([]RowEntry, 0, row.size)
	for i := range row.entries {
		if !row.entries[i].dead {
			out = append(out, RowEntry{row.entries[i].variable, new(big.Rat).Set(&row.entries[i].coeff)})
		}
	}
	return out, nil
}

func (m *Matrix) coefficient(r Row, variable uint32) (*big.Rat, bool) {
	if !m.valid(r) {
		return nil, false
	}
	if i := m.entryIndex(r, variable); i >= 0 {
		return &m.rows[r.id].entries[i].coeff, true
	}
	return nil, false
}

// Add sets destination = destination + coefficient*source, following
// sparse_matrix::add (ADD_ROW): positions of destination variables are
// saved first, missing variables take free slots, cancelled entries are
// deleted at once, and the destination is compressed if needed.
func (m *Matrix) Add(destination Row, coefficient *big.Rat, source Row) error {
	if !m.valid(destination) || !m.valid(source) || coefficient == nil {
		return fmt.Errorf("simplex: invalid row addition")
	}
	m.addRows++
	if coefficient.Sign() == 0 {
		return nil
	}
	positions := make(map[uint32]int, m.rows[destination.id].size)
	for i := range m.rows[destination.id].entries {
		e := &m.rows[destination.id].entries[i]
		if !e.dead {
			positions[e.variable] = i
		}
	}
	src := &m.rows[source.id]
	for i := 0; i < len(src.entries); i++ {
		item := &src.entries[i]
		if item.dead {
			continue
		}
		term := new(big.Rat).Mul(&item.coeff, coefficient)
		pos, found := positions[item.variable]
		if !found {
			m.addVar(destination.id, term, item.variable)
			continue
		}
		current := &m.rows[destination.id].entries[pos].coeff
		current.Add(current, term)
		if current.Sign() == 0 {
			m.deleteRowEntry(destination.id, pos)
		}
	}
	if row := &m.rows[destination.id]; row.size*2 < len(row.entries) {
		m.compressRow(destination.id)
	}
	return nil
}

func (m *Matrix) Multiply(r Row, coefficient *big.Rat) error {
	if !m.valid(r) || coefficient == nil || coefficient.Sign() == 0 {
		return fmt.Errorf("simplex: invalid row multiplier")
	}
	if coefficient.Cmp(big.NewRat(1, 1)) == 0 {
		return nil
	}
	for i := range m.rows[r.id].entries {
		if !m.rows[r.id].entries[i].dead {
			m.rows[r.id].entries[i].coeff.Mul(&m.rows[r.id].entries[i].coeff, coefficient)
		}
	}
	return nil
}
func (m *Matrix) Negate(r Row) error { return m.Multiply(r, big.NewRat(-1, 1)) }

// Delete is sparse_matrix::del: every live entry is deleted in slot order
// and the row id is recycled; the row keeps its slots and free list.
func (m *Matrix) Delete(r Row) error {
	if !m.valid(r) {
		return fmt.Errorf("simplex: invalid row")
	}
	for i := range m.rows[r.id].entries {
		if !m.rows[r.id].entries[i].dead {
			m.deleteRowEntry(r.id, i)
		}
	}
	m.rows[r.id].live = false
	m.dead = append(m.dead, r.id)
	return nil
}

func gcdInt(a, b *big.Int) *big.Int {
	a = new(big.Int).Abs(new(big.Int).Set(a))
	b = new(big.Int).Abs(new(big.Int).Set(b))
	return new(big.Int).GCD(nil, nil, a, b)
}
func lcmInt(a, b *big.Int) *big.Int {
	if a.Sign() == 0 || b.Sign() == 0 {
		return new(big.Int)
	}
	g := gcdInt(a, b)
	return new(big.Int).Abs(new(big.Int).Mul(new(big.Int).Quo(new(big.Int).Set(a), g), b))
}

// GCDNormalize divides a row by the exact rational gcd of its coefficients
// and returns that gcd. For integral rows this is the ordinary positive GCD.
func (m *Matrix) GCDNormalize(r Row) (*big.Rat, error) {
	if !m.valid(r) {
		return nil, fmt.Errorf("simplex: invalid row")
	}
	row := &m.rows[r.id]
	if row.size == 0 {
		return big.NewRat(1, 1), nil
	}
	num := new(big.Int)
	den := big.NewInt(1)
	for i := range row.entries {
		if row.entries[i].dead {
			continue
		}
		c := &row.entries[i].coeff
		if num.Sign() == 0 {
			num.Abs(c.Num())
		} else {
			num = gcdInt(num, c.Num())
		}
		den = lcmInt(den, c.Denom())
	}
	if num.Sign() == 0 {
		return big.NewRat(1, 1), nil
	}
	g := new(big.Rat).SetFrac(num, den)
	if g.Cmp(big.NewRat(1, 1)) != 0 {
		for i := range row.entries {
			if !row.entries[i].dead {
				row.entries[i].coeff.Quo(&row.entries[i].coeff, g)
			}
		}
	}
	return g, nil
}

func (m *Matrix) ColumnSize(variable uint32) uint32 {
	if int(variable) >= len(m.columns) {
		return 0
	}
	return uint32(m.columns[variable].size)
}

// ColumnRows lists the column's live rows in slot order (col_iterator).
func (m *Matrix) ColumnRows(variable uint32) []Row {
	if int(variable) >= len(m.columns) {
		return nil
	}
	c := &m.columns[variable]
	out := make([]Row, 0, c.size)
	for i := range c.entries {
		if !c.entries[i].dead {
			out = append(out, Row{c.entries[i].row})
		}
	}
	return out
}
func (m *Matrix) AddRowCount() uint32 { return m.addRows }

func (m *Matrix) Reset() { *m = Matrix{} }

func (m *Matrix) WellFormed() bool {
	for rid := range m.rows {
		row := &m.rows[rid]
		if !row.live {
			continue
		}
		seen := make(map[uint32]bool)
		live := 0
		for i := range row.entries {
			e := &row.entries[i]
			if e.dead {
				continue
			}
			live++
			if e.coeff.Sign() == 0 || seen[e.variable] || int(e.variable) >= len(m.columns) {
				return false
			}
			seen[e.variable] = true
			c := &m.columns[e.variable]
			if e.colIdx < 0 || e.colIdx >= len(c.entries) || c.entries[e.colIdx].dead ||
				c.entries[e.colIdx].row != uint32(rid) || c.entries[e.colIdx].rowIdx != i {
				return false
			}
		}
		if live != row.size {
			return false
		}
	}
	for v := range m.columns {
		c := &m.columns[v]
		seen := make(map[uint32]bool)
		live := 0
		for i := range c.entries {
			e := &c.entries[i]
			if e.dead {
				continue
			}
			live++
			if seen[e.row] || int(e.row) >= len(m.rows) || !m.rows[e.row].live {
				return false
			}
			seen[e.row] = true
			row := &m.rows[e.row]
			if e.rowIdx < 0 || e.rowIdx >= len(row.entries) || row.entries[e.rowIdx].dead ||
				row.entries[e.rowIdx].variable != uint32(v) || row.entries[e.rowIdx].colIdx != i {
				return false
			}
		}
		if live != c.size {
			return false
		}
	}
	return true
}

func (m *Matrix) DisplayRow(w io.Writer, r Row) error {
	entries, err := m.Entries(r)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if _, err = fmt.Fprintf(w, "%s*v%d ", e.Coefficient.RatString(), e.Variable); err != nil {
			return err
		}
	}
	_, err = io.WriteString(w, "\n")
	return err
}
func (m *Matrix) Display(w io.Writer) error {
	for i := range m.rows {
		if m.rows[i].live && m.rows[i].size != 0 {
			if err := m.DisplayRow(w, Row{uint32(i)}); err != nil {
				return err
			}
		}
	}
	return nil
}
