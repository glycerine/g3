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

// Source: src/math/simplex/sparse_matrix.h. A row or column keeps dead
// entries in place, chained through a LIFO free list (the source unions
// m_col_idx/m_next_free_row_entry_idx and m_row_idx/m_next_free_col_entry_idx;
// colIdx and rowIdx play both roles here). Iteration is in slot order, so
// reused holes decide the visiting order, exactly as row_iterator and
// col_iterator do.
const matrixDeadID = ^uint32(0)

// matrixEntry is _row_entry: variable*coeff. variable is matrixDeadID for a
// dead entry, and colIdx then holds the next free row entry index.
type matrixEntry struct {
	variable uint32
	coeff    big.Rat
	colIdx   int32
}

func (e *matrixEntry) isDead() bool { return e.variable == matrixDeadID }

// matrixRow is _row. live is the Go validity flag for the public Row API;
// the source keeps no such flag and a deleted row keeps its dead entries and
// free list until mk_row hands it out again.
type matrixRow struct {
	entries   []matrixEntry
	size      uint32
	firstFree int32
	live      bool
}

// matrixColumnEntry is col_entry. rowID is matrixDeadID for a dead entry,
// and rowIdx then holds the next free column entry index.
type matrixColumnEntry struct {
	rowID  uint32
	rowIdx int32
}

func (e *matrixColumnEntry) isDead() bool { return e.rowID == matrixDeadID }

// matrixColumn is column. refs counts outstanding column iterations, which
// block compression as m_refs does.
type matrixColumn struct {
	entries   []matrixColumnEntry
	size      uint32
	firstFree int32
	refs      uint32
}

// Matrix is a concrete exact-rational replacement for the two retained C++
// sparse_matrix instantiations. Integer coefficients use denominator one.
type Matrix struct {
	rows      []matrixRow
	deadRows  []uint32
	columns   []matrixColumn
	varPos    []int32
	varPosIdx []uint32
	addRows   uint32
}

// addRowEntry is _row::add_row_entry.
func (r *matrixRow) addRowEntry() int32 {
	r.size++
	if r.firstFree == -1 {
		r.entries = append(r.entries, matrixEntry{variable: matrixDeadID})
		return int32(len(r.entries) - 1)
	}
	pos := r.firstFree
	result := &r.entries[pos]
	r.firstFree = result.colIdx
	return pos
}

// delRowEntry is _row::del_row_entry.
func (r *matrixRow) delRowEntry(idx int32) {
	t := &r.entries[idx]
	t.colIdx = r.firstFree
	t.variable = matrixDeadID
	r.size--
	r.firstFree = idx
}

// compress is _row::compress.
func (r *matrixRow) compress(cols []matrixColumn) {
	j := 0
	for i := range r.entries {
		t1 := &r.entries[i]
		if !t1.isDead() {
			if i != j {
				t2 := &r.entries[j]
				t2.coeff, t1.coeff = t1.coeff, t2.coeff
				t2.variable = t1.variable
				t2.colIdx = t1.colIdx
				cols[t2.variable].entries[t2.colIdx].rowIdx = int32(j)
			}
			j++
		}
	}
	for i := int(r.size); i < len(r.entries); i++ {
		r.entries[i] = matrixEntry{}
	}
	r.entries = r.entries[:r.size]
	r.firstFree = -1
}

// compressIfNeeded is _row::compress_if_needed.
func (r *matrixRow) compressIfNeeded(cols []matrixColumn) {
	if r.size*2 < uint32(len(r.entries)) {
		r.compress(cols)
	}
}

// indexOf is _row::get_idx_of.
func (r *matrixRow) indexOf(v uint32) int32 {
	for i := range r.entries {
		if !r.entries[i].isDead() && r.entries[i].variable == v {
			return int32(i)
		}
	}
	return -1
}

// addColumnEntry is column::add_col_entry.
func (c *matrixColumn) addColumnEntry() int32 {
	c.size++
	if c.firstFree == -1 {
		c.entries = append(c.entries, matrixColumnEntry{})
		return int32(len(c.entries) - 1)
	}
	pos := c.firstFree
	c.firstFree = c.entries[pos].rowIdx
	return pos
}

// delColumnEntry is column::del_col_entry.
func (c *matrixColumn) delColumnEntry(idx int32) {
	e := &c.entries[idx]
	e.rowID = matrixDeadID
	e.rowIdx = c.firstFree
	c.firstFree = idx
	c.size--
}

// compress is column::compress.
func (c *matrixColumn) compress(rows []matrixRow) {
	j := 0
	for i := range c.entries {
		e1 := c.entries[i]
		if !e1.isDead() {
			if i != j {
				c.entries[j] = e1
				rows[e1.rowID].entries[e1.rowIdx].colIdx = int32(j)
			}
			j++
		}
	}
	c.entries = c.entries[:c.size]
	c.firstFree = -1
}

// compressIfNeeded is column::compress_if_needed.
func (c *matrixColumn) compressIfNeeded(rows []matrixRow) {
	if c.size*2 < uint32(len(c.entries)) && c.refs == 0 {
		c.compress(rows)
	}
}

// EnsureVariable is sparse_matrix::ensure_var.
func (m *Matrix) EnsureVariable(v uint32) {
	for uint32(len(m.columns)) <= v {
		m.columns = append(m.columns, matrixColumn{firstFree: -1})
		m.varPos = append(m.varPos, -1)
	}
}

// NewRow is sparse_matrix::mk_row. A recycled row keeps its dead entries and
// free list, so its next entries reuse the most recently freed slots.
func (m *Matrix) NewRow() Row {
	if len(m.deadRows) != 0 {
		i := m.deadRows[len(m.deadRows)-1]
		m.deadRows = m.deadRows[:len(m.deadRows)-1]
		m.rows[i].live = true
		return Row{i}
	}
	r := Row{uint32(len(m.rows))}
	m.rows = append(m.rows, matrixRow{firstFree: -1, live: true})
	return r
}

func (m *Matrix) valid(r Row) bool { return int(r.id) < len(m.rows) && m.rows[r.id].live }

// retainColumn and releaseColumn bracket a column traversal that may delete
// entries of that column, mirroring a live col_iterator's m_refs.
func (m *Matrix) retainColumn(v uint32)  { m.columns[v].refs++ }
func (m *Matrix) releaseColumn(v uint32) { m.columns[v].refs-- }

// AddRational is sparse_matrix::add_var.
func (m *Matrix) AddRational(r Row, coefficient *big.Rat, variable uint32) error {
	if !m.valid(r) || coefficient == nil || coefficient.Sign() == 0 {
		return fmt.Errorf("simplex: invalid matrix insertion")
	}
	m.EnsureVariable(variable)
	if m.rows[r.id].indexOf(variable) != -1 {
		return fmt.Errorf("simplex: duplicate variable %d", variable)
	}
	row := &m.rows[r.id]
	c := &m.columns[variable]
	rIdx := row.addRowEntry()
	cIdx := c.addColumnEntry()
	rEntry := &row.entries[rIdx]
	rEntry.variable = variable
	rEntry.coeff.Set(coefficient)
	rEntry.colIdx = cIdx
	c.entries[cIdx] = matrixColumnEntry{rowID: r.id, rowIdx: rIdx}
	return nil
}

func (m *Matrix) AddInteger(r Row, coefficient *big.Int, variable uint32) error {
	if coefficient == nil {
		return fmt.Errorf("simplex: nil coefficient")
	}
	return m.AddRational(r, new(big.Rat).SetInt(coefficient), variable)
}

// Entries returns the live entries of r in slot order (row_iterator).
func (m *Matrix) Entries(r Row) ([]RowEntry, error) {
	if !m.valid(r) {
		return nil, fmt.Errorf("simplex: invalid row")
	}
	row := &m.rows[r.id]
	out := make([]RowEntry, 0, row.size)
	for i := range row.entries {
		e := &row.entries[i]
		if !e.isDead() {
			out = append(out, RowEntry{e.variable, new(big.Rat).Set(&e.coeff)})
		}
	}
	return out, nil
}

func (m *Matrix) coefficient(r Row, variable uint32) (*big.Rat, bool) {
	if !m.valid(r) {
		return nil, false
	}
	if i := m.rows[r.id].indexOf(variable); i != -1 {
		return &m.rows[r.id].entries[i].coeff, true
	}
	return nil, false
}

// saveVarPos is _row::save_var_pos.
func (m *Matrix) saveVarPos(r *matrixRow) {
	for i := range r.entries {
		if !r.entries[i].isDead() {
			m.varPos[r.entries[i].variable] = int32(i)
			m.varPosIdx = append(m.varPosIdx, r.entries[i].variable)
		}
	}
}

// Add sets destination = destination + coefficient*source, following
// sparse_matrix::add and its ADD_ROW loop: source entries are visited in
// slot order, new variables take the destination's free slots first, and
// cancelled entries are freed immediately.
func (m *Matrix) Add(destination Row, coefficient *big.Rat, source Row) error {
	if !m.valid(destination) || !m.valid(source) || coefficient == nil {
		return fmt.Errorf("simplex: invalid row addition")
	}
	m.addRows++
	if coefficient.Sign() == 0 {
		return nil
	}
	r1 := &m.rows[destination.id]
	m.saveVarPos(r1)
	src := &m.rows[source.id]
	for i := 0; i < len(src.entries); i++ {
		it := &src.entries[i]
		if it.isDead() {
			continue
		}
		v := it.variable
		pos := m.varPos[v]
		if pos == -1 {
			// variable v is not in row1
			rowIdx := r1.addRowEntry()
			rEntry := &r1.entries[rowIdx]
			rEntry.variable = v
			rEntry.coeff.Mul(&it.coeff, coefficient)
			c := &m.columns[v]
			colIdx := c.addColumnEntry()
			rEntry.colIdx = colIdx
			c.entries[colIdx] = matrixColumnEntry{rowID: destination.id, rowIdx: rowIdx}
		} else {
			// variable v is in row1
			rEntry := &r1.entries[pos]
			rEntry.coeff.Add(&rEntry.coeff, new(big.Rat).Mul(&it.coeff, coefficient))
			if rEntry.coeff.Sign() == 0 {
				m.delRowEntry(r1, pos)
			}
		}
	}
	for _, v := range m.varPosIdx {
		m.varPos[v] = -1
	}
	m.varPosIdx = m.varPosIdx[:0]
	r1.compressIfNeeded(m.columns)
	return nil
}

// delRowEntry is sparse_matrix::del_row_entry.
func (m *Matrix) delRowEntry(r *matrixRow, pos int32) {
	rEntry := &r.entries[pos]
	v := rEntry.variable
	colIdx := rEntry.colIdx
	r.delRowEntry(pos)
	c := &m.columns[v]
	c.delColumnEntry(colIdx)
	c.compressIfNeeded(m.rows)
}

func (m *Matrix) Multiply(r Row, coefficient *big.Rat) error {
	if !m.valid(r) || coefficient == nil || coefficient.Sign() == 0 {
		return fmt.Errorf("simplex: invalid row multiplier")
	}
	if coefficient.Cmp(big.NewRat(1, 1)) == 0 {
		return nil
	}
	row := &m.rows[r.id]
	for i := range row.entries {
		if !row.entries[i].isDead() {
			row.entries[i].coeff.Mul(&row.entries[i].coeff, coefficient)
		}
	}
	return nil
}
func (m *Matrix) Negate(r Row) error { return m.Multiply(r, big.NewRat(-1, 1)) }

// Delete is sparse_matrix::del. The row keeps its dead entries and free list
// for the next mk_row.
func (m *Matrix) Delete(r Row) error {
	if !m.valid(r) {
		return fmt.Errorf("simplex: invalid row")
	}
	rw := &m.rows[r.id]
	for i := 0; i < len(rw.entries); i++ {
		if !rw.entries[i].isDead() {
			m.delRowEntry(rw, int32(i))
		}
	}
	rw.live = false
	m.deadRows = append(m.deadRows, r.id)
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
		if row.entries[i].isDead() {
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
			if !row.entries[i].isDead() {
				row.entries[i].coeff.Quo(&row.entries[i].coeff, g)
			}
		}
	}
	return g, nil
}

// ColumnSize is sparse_matrix::column_size.
func (m *Matrix) ColumnSize(variable uint32) uint32 {
	if int(variable) >= len(m.columns) {
		return 0
	}
	return m.columns[variable].size
}

// ColumnRows returns the rows of the live column entries in slot order
// (col_iterator).
func (m *Matrix) ColumnRows(variable uint32) []Row {
	if int(variable) >= len(m.columns) {
		return nil
	}
	c := &m.columns[variable]
	out := make([]Row, 0, c.size)
	for i := range c.entries {
		if !c.entries[i].isDead() {
			out = append(out, Row{c.entries[i].rowID})
		}
	}
	return out
}
func (m *Matrix) AddRowCount() uint32 { return m.addRows }

func (m *Matrix) Reset() { *m = Matrix{} }

// WellFormed follows sparse_matrix::well_formed_row and well_formed_column,
// including their free-list checks.
func (m *Matrix) WellFormed() bool {
	for rid := range m.rows {
		r := &m.rows[rid]
		vars := make(map[uint32]bool)
		dead := make(map[int32]bool)
		var live uint32
		for i := range r.entries {
			e := &r.entries[i]
			if e.isDead() {
				dead[int32(i)] = true
				continue
			}
			if vars[e.variable] || e.coeff.Sign() == 0 || int(e.variable) >= len(m.columns) {
				return false
			}
			col := &m.columns[e.variable]
			if e.colIdx < 0 || int(e.colIdx) >= len(col.entries) {
				return false
			}
			c := &col.entries[e.colIdx]
			if c.rowID != uint32(rid) || c.rowIdx != int32(i) {
				return false
			}
			vars[e.variable] = true
			live++
		}
		if live != r.size || (!r.live && live != 0) {
			return false
		}
		for idx := r.firstFree; idx != -1; idx = r.entries[idx].colIdx {
			if !dead[idx] {
				return false
			}
			delete(dead, idx)
		}
		if len(dead) != 0 {
			return false
		}
	}
	for v := range m.columns {
		col := &m.columns[v]
		rows := make(map[uint32]bool)
		dead := make(map[int32]bool)
		var live uint32
		for i := range col.entries {
			c := &col.entries[i]
			if c.isDead() {
				dead[int32(i)] = true
				continue
			}
			if rows[c.rowID] || int(c.rowID) >= len(m.rows) || !m.rows[c.rowID].live {
				return false
			}
			row := &m.rows[c.rowID]
			if c.rowIdx < 0 || int(c.rowIdx) >= len(row.entries) {
				return false
			}
			r := &row.entries[c.rowIdx]
			if r.variable != uint32(v) || r.colIdx != int32(i) {
				return false
			}
			rows[c.rowID] = true
			live++
		}
		if live != col.size {
			return false
		}
		for idx := col.firstFree; idx != -1; idx = col.entries[idx].rowIdx {
			if !dead[idx] {
				return false
			}
			delete(dead, idx)
		}
		if len(dead) != 0 {
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
		if m.rows[i].size != 0 {
			if err := m.DisplayRow(w, Row{uint32(i)}); err != nil {
				return err
			}
		}
	}
	return nil
}
