package smtcore

// Source: src/smt/theory_arith.h:97-216 and
// src/smt/theory_arith_aux.h:29-330.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

const smtArithmeticDeadRow = -1

type smtArithmeticRowEntry struct {
	coefficient numeral.Rational
	variable    SMTTheoryVar
	columnIndex int
	freeNext    int
}

func (e *smtArithmeticRowEntry) dead() bool { return e.variable == SMTNullTheoryVar }

type smtArithmeticColumnEntry struct {
	rowID    int
	rowIndex int
	freeNext int
}

func (e *smtArithmeticColumnEntry) dead() bool { return e.rowID == smtArithmeticDeadRow }

type smtArithmeticRow struct {
	entries   []smtArithmeticRowEntry
	size      int
	base      SMTTheoryVar
	firstFree int
}

func newSMTArithmeticRow() smtArithmeticRow {
	return smtArithmeticRow{base: SMTNullTheoryVar, firstFree: -1}
}

// reset frees the live coefficients (a dead entry's was freed by
// deleteEntry) and empties the row.
func (r *smtArithmeticRow) reset(q *numeral.RationalManager) error {
	var err error
	for index := range r.entries {
		if e := smtQDel(q, &r.entries[index].coefficient); e != nil && err == nil {
			err = e
		}
	}
	r.entries = r.entries[:0]
	r.size, r.base, r.firstFree = 0, SMTNullTheoryVar, -1
	return err
}

func (r *smtArithmeticRow) addEntry() (int, *smtArithmeticRowEntry) {
	r.size++
	if r.firstFree < 0 {
		index := len(r.entries)
		r.entries = append(r.entries, smtArithmeticRowEntry{coefficient: smtQZero(), variable: SMTNullTheoryVar, freeNext: -1})
		return index, &r.entries[index]
	}
	index := r.firstFree
	entry := &r.entries[index]
	r.firstFree = entry.freeNext
	entry.freeNext = -1
	return index, entry
}

func (r *smtArithmeticRow) deleteEntry(q *numeral.RationalManager, index int) error {
	if index < 0 || index >= len(r.entries) || r.entries[index].dead() {
		return ast.ErrASTDomain
	}
	// row::del_row_entry links the entry to the free list but, unlike
	// column::del_col_entry, never makes it the list head: a dead row slot
	// is not reused before compress, and later entries are appended.
	entry := &r.entries[index]
	entry.variable, entry.freeNext = SMTNullTheoryVar, r.firstFree
	r.size--
	return q.Reset(&entry.coefficient)
}

func (r *smtArithmeticRow) compress(columns []smtArithmeticColumn) error {
	target := 0
	for source := range r.entries {
		entry := &r.entries[source]
		if entry.dead() {
			continue
		}
		if int(entry.variable) < 0 || int(entry.variable) >= len(columns) || entry.columnIndex < 0 || entry.columnIndex >= len(columns[entry.variable].entries) {
			return ast.ErrASTDomain
		}
		if source != target {
			// A move: the source slot's coefficient now belongs to target.
			r.entries[target] = *entry
			columns[entry.variable].entries[entry.columnIndex].rowIndex = target
		}
		target++
	}
	if target != r.size {
		return ast.ErrASTDomain
	}
	for index := target; index < len(r.entries); index++ {
		r.entries[index] = smtArithmeticRowEntry{}
	}
	r.entries, r.firstFree = r.entries[:target], -1
	return nil
}

func (r *smtArithmeticRow) compressIfNeeded(columns []smtArithmeticColumn) error {
	if r.size*2 < len(r.entries) {
		return r.compress(columns)
	}
	return nil
}

func (r *smtArithmeticRow) saveVariablePositions(positions []int) error {
	for index := range r.entries {
		entry := &r.entries[index]
		if entry.dead() {
			continue
		}
		if int(entry.variable) < 0 || int(entry.variable) >= len(positions) {
			return ast.ErrASTDomain
		}
		positions[entry.variable] = index
	}
	return nil
}

func (r *smtArithmeticRow) resetVariablePositions(positions []int) error {
	for index := range r.entries {
		entry := &r.entries[index]
		if entry.dead() {
			continue
		}
		if int(entry.variable) < 0 || int(entry.variable) >= len(positions) {
			return ast.ErrASTDomain
		}
		positions[entry.variable] = -1
	}
	return nil
}

// denominatorLCM is row::get_denominators_lcm: out = lcm of the
// denominators of the live coefficients. An integer coefficient's
// denominator 1 leaves the positive lcm unchanged, so it is skipped.
func (r *smtArithmeticRow) denominatorLCM(q *numeral.RationalManager, out *numeral.Rational) error {
	if err := q.SetInt64(out, 1); err != nil {
		return err
	}
	den := smtQZero()
	defer smtQDel(q, &den)
	for index := range r.entries {
		entry := &r.entries[index]
		if !entry.dead() && !entry.coefficient.IsInt() {
			if err := q.DenominatorRational(&entry.coefficient, &den); err != nil {
				return err
			}
			if err := q.LCM(out, &den, out); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *smtArithmeticRow) indexOf(variable SMTTheoryVar) int {
	for index := range r.entries {
		if !r.entries[index].dead() && r.entries[index].variable == variable {
			return index
		}
	}
	return -1
}

type smtArithmeticColumn struct {
	entries   []smtArithmeticColumnEntry
	size      int
	firstFree int
}

func newSMTArithmeticColumn() smtArithmeticColumn { return smtArithmeticColumn{firstFree: -1} }

func (c *smtArithmeticColumn) addEntry() (int, *smtArithmeticColumnEntry) {
	c.size++
	if c.firstFree < 0 {
		index := len(c.entries)
		c.entries = append(c.entries, smtArithmeticColumnEntry{rowID: smtArithmeticDeadRow, freeNext: -1})
		return index, &c.entries[index]
	}
	index := c.firstFree
	entry := &c.entries[index]
	c.firstFree = entry.freeNext
	entry.freeNext = -1
	return index, entry
}

func (c *smtArithmeticColumn) deleteEntry(index int) error {
	if index < 0 || index >= len(c.entries) || c.entries[index].dead() {
		return ast.ErrASTDomain
	}
	entry := &c.entries[index]
	entry.rowID, entry.freeNext = smtArithmeticDeadRow, c.firstFree
	c.firstFree, c.size = index, c.size-1
	return nil
}

func (c *smtArithmeticColumn) compress(rows []smtArithmeticRow) error {
	target := 0
	for source := range c.entries {
		entry := c.entries[source]
		if entry.dead() {
			continue
		}
		if entry.rowID < 0 || entry.rowID >= len(rows) || entry.rowIndex < 0 || entry.rowIndex >= len(rows[entry.rowID].entries) {
			return ast.ErrASTDomain
		}
		if source != target {
			c.entries[target] = entry
			rows[entry.rowID].entries[entry.rowIndex].columnIndex = target
		}
		target++
	}
	if target != c.size {
		return ast.ErrASTDomain
	}
	clear(c.entries[target:])
	c.entries, c.firstFree = c.entries[:target], -1
	return nil
}

func (c *smtArithmeticColumn) compressIfNeeded(rows []smtArithmeticRow) error {
	if c.size*2 < len(c.entries) {
		return c.compress(rows)
	}
	return nil
}

func (c *smtArithmeticColumn) compressSingleton(rows []smtArithmeticRow, singleton int) error {
	if c.size != 1 || singleton < 0 || singleton >= len(c.entries) || c.entries[singleton].dead() {
		return ast.ErrASTDomain
	}
	entry := c.entries[singleton]
	if entry.rowID < 0 || entry.rowID >= len(rows) || entry.rowIndex < 0 || entry.rowIndex >= len(rows[entry.rowID].entries) {
		return ast.ErrASTDomain
	}
	if singleton != 0 {
		c.entries[0] = entry
		rows[entry.rowID].entries[entry.rowIndex].columnIndex = 0
	}
	clear(c.entries[1:])
	c.entries, c.firstFree = c.entries[:1], -1
	return nil
}
