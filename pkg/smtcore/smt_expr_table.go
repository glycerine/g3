package smtcore

// Source: context::m_expr2bool_var, m_app2enode, cached generations and
// relevance marks. Each solver indexes its own pages by AST slot.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

const (
	smtExprHasBoolVar uint8 = 1 << iota
	smtExprHasAssignment
	smtExprHasRelevant
	smtExprRelevant
	smtExprHasCachedGeneration
	smtExprCacheGenerationVisited
)

type smtExprRecord struct {
	assignment int8 // SetAssignment accepts the three lbool values.
	flags      uint8
}

const smtExprPageSize = 256

// Keeping the pointers together shortens the region the GC must scan.
// A page allocates cached generations on the first cache write.
type smtExprPage struct {
	cached      *[smtExprPageSize]uint32
	enodes      [smtExprPageSize]*SMTEnode
	generations [smtExprPageSize]uint32
	variables   [smtExprPageSize]int32
	records     [smtExprPageSize]smtExprRecord
}

// smtExprTable is owned by one SMTCheckerContext and borrows nothing. A record
// whose generation differs from a handle's describes a retired expression.
// Only touched pages are allocated. Their records stay in slot order.
type smtExprTable struct {
	pages              []*smtExprPage
	origin             uint64
	enodeCount         int
	boolVarCount       int
	relevanceRemovals  uint64
	relevanceAdditions uint64
}

func newSMTExprTable() *smtExprTable { return &smtExprTable{} }

// read returns the live record for a handle, or nil.
func (t *smtExprTable) read(h ast.ASTHandle) *smtExprRecord {
	page := t.readPage(h)
	if page == nil {
		return nil
	}
	return &page.records[h.Slot()%smtExprPageSize]
}

func (t *smtExprTable) readPage(h ast.ASTHandle) *smtExprPage {
	slot := h.Slot()
	index := slot / smtExprPageSize
	if uint64(index) >= uint64(len(t.pages)) || h.Origin() != t.origin {
		return nil
	}
	page := t.pages[index]
	if page == nil || page.generations[slot%smtExprPageSize] != h.Generation() {
		return nil
	}
	return page
}

// write returns the record for a handle, retiring any older generation's state.
func (t *smtExprTable) write(h ast.ASTHandle) *smtExprRecord {
	page := t.writePage(h)
	return &page.records[h.Slot()%smtExprPageSize]
}

func (t *smtExprTable) writePage(h ast.ASTHandle) *smtExprPage {
	if t.origin == 0 {
		t.origin = h.Origin()
	} else if h.Origin() != t.origin {
		panic("smtcore: expression table used with a foreign manager's handle")
	}
	slot := h.Slot()
	index := int(slot / smtExprPageSize)
	if index >= len(t.pages) {
		t.pages = append(t.pages, make([]*smtExprPage, index+1-len(t.pages))...)
	}
	page := t.pages[index]
	if page == nil {
		page = new(smtExprPage)
		t.pages[index] = page
	}
	offset := slot % smtExprPageSize
	record := &page.records[offset]
	if page.generations[offset] != h.Generation() {
		if record.flags&smtExprRelevant != 0 {
			t.relevanceRemovals++
		}
		if page.enodes[offset] != nil {
			page.enodes[offset] = nil
			t.enodeCount--
		}
		if record.flags&smtExprHasBoolVar != 0 {
			t.boolVarCount--
		}
		*record = smtExprRecord{}
		page.generations[offset] = h.Generation()
		page.variables[offset] = 0
	}
	return page
}

type smtEnodeTable struct{ t *smtExprTable }

func (e smtEnodeTable) get(h ast.ASTHandle) *SMTEnode {
	if page := e.t.readPage(h); page != nil {
		return page.enodes[h.Slot()%smtExprPageSize]
	}
	return nil
}
func (e smtEnodeTable) lookup(h ast.ASTHandle) (*SMTEnode, bool) {
	node := e.get(h)
	return node, node != nil
}
func (e smtEnodeTable) set(h ast.ASTHandle, node *SMTEnode) {
	page := e.t.writePage(h)
	slot := &page.enodes[h.Slot()%smtExprPageSize]
	if *slot == nil && node != nil {
		e.t.enodeCount++
	} else if *slot != nil && node == nil {
		e.t.enodeCount--
	}
	*slot = node
}
func (e smtEnodeTable) del(h ast.ASTHandle) {
	if page := e.t.readPage(h); page != nil && page.enodes[h.Slot()%smtExprPageSize] != nil {
		page.enodes[h.Slot()%smtExprPageSize] = nil
		e.t.enodeCount--
	}
}
func (e smtEnodeTable) count() int { return e.t.enodeCount }

// all lists live enodes in slot order, which is deterministic.
func (e smtEnodeTable) all() []*SMTEnode {
	result := make([]*SMTEnode, 0, e.t.enodeCount)
	for _, page := range e.t.pages {
		if page == nil {
			continue
		}
		for i := range page.enodes {
			if node := page.enodes[i]; node != nil {
				result = append(result, node)
			}
		}
	}
	return result
}

// variableAt reads a slot whose Boolean presence flag has already been checked.
func (t *smtExprTable) variableAt(slot uint32) int32 {
	return t.pages[slot/smtExprPageSize].variables[slot%smtExprPageSize]
}

type smtBoolVarTable struct{ t *smtExprTable }

func (b smtBoolVarTable) lookup(h ast.ASTHandle) (int32, bool) {
	if page := b.t.readPage(h); page != nil && page.records[h.Slot()%smtExprPageSize].flags&smtExprHasBoolVar != 0 {
		return page.variables[h.Slot()%smtExprPageSize], true
	}
	return 0, false
}
func (b smtBoolVarTable) get(h ast.ASTHandle) int32 {
	variable, _ := b.lookup(h)
	return variable
}
func (b smtBoolVarTable) set(h ast.ASTHandle, variable int32) {
	page := b.t.writePage(h)
	offset := h.Slot() % smtExprPageSize
	record := &page.records[offset]
	if record.flags&smtExprHasBoolVar == 0 {
		b.t.boolVarCount++
	}
	page.variables[offset] = variable
	record.flags |= smtExprHasBoolVar
}
func (b smtBoolVarTable) del(h ast.ASTHandle) {
	if record := b.t.read(h); record != nil && record.flags&smtExprHasBoolVar != 0 {
		record.flags &^= smtExprHasBoolVar
		b.t.boolVarCount--
	}
}

// retireBoolean clears this variable's state with one expression-table lookup.
func (t *smtExprTable) retireBoolean(h ast.ASTHandle, clearRelevance bool) {
	record := t.read(h)
	if record == nil {
		return
	}
	if record.flags&smtExprHasBoolVar != 0 {
		t.boolVarCount--
	}
	record.flags &^= smtExprHasBoolVar | smtExprHasAssignment
	record.assignment = 0
	if clearRelevance {
		if record.flags&smtExprRelevant != 0 {
			t.relevanceRemovals++
		}
		record.flags &^= smtExprHasRelevant | smtExprRelevant
	}
}

func (b smtBoolVarTable) count() int { return b.t.boolVarCount }

// smtAssignmentTable answers context::b_internalized and get_assignment_core
// for an expression. When the context has an internalizer (s), it reads the
// expression's Boolean variable and that variable's value in m_assignment,
// so lookup reports found exactly when the expression is internalized as a
// Boolean (b_internalized), whether or not it was ever assigned. The
// internalizer does not copy its values here. A value set through
// SMTCheckerContext.SetAssignment (a standalone view, s nil, or a fixture
// that drives the context directly) is kept in the record and read first.
type smtAssignmentTable struct {
	t *smtExprTable
	s *SMTInternalizerState
}

func (a smtAssignmentTable) lookup(h ast.ASTHandle) (lbool.LBool, bool) {
	return a.lookupRecord(h.Slot(), a.t.read(h))
}

func (a smtAssignmentTable) lookupRecord(slot uint32, record *smtExprRecord) (lbool.LBool, bool) {
	if record == nil {
		return 0, false
	}
	if record.flags&smtExprHasAssignment != 0 {
		return lbool.LBool(record.assignment), true
	}
	if a.s != nil && record.flags&smtExprHasBoolVar != 0 {
		return a.s.assignment[a.t.variableAt(slot)<<1], true
	}
	return 0, false
}
func (a smtAssignmentTable) get(h ast.ASTHandle) lbool.LBool {
	value, _ := a.lookup(h)
	return value
}
func (a smtAssignmentTable) set(h ast.ASTHandle, value lbool.LBool) {
	record := a.t.write(h)
	record.assignment = int8(value)
	record.flags |= smtExprHasAssignment
}
func (a smtAssignmentTable) del(h ast.ASTHandle) {
	if record := a.t.read(h); record != nil {
		record.flags &^= smtExprHasAssignment
		record.assignment = 0
	}
}

type smtRelevantTable struct{ t *smtExprTable }

func (r smtRelevantTable) lookup(h ast.ASTHandle) (bool, bool) {
	if record := r.t.read(h); record != nil && record.flags&smtExprHasRelevant != 0 {
		return record.flags&smtExprRelevant != 0, true
	}
	return false, false
}
func (r smtRelevantTable) get(h ast.ASTHandle) bool {
	record := r.t.read(h)
	return record != nil && record.flags&smtExprRelevant != 0
}

// set returns whether the expression already had a live relevance mark.
func (r smtRelevantTable) set(h ast.ASTHandle, value bool) bool {
	record := r.t.write(h)
	marked := record.flags&smtExprRelevant != 0
	record.flags |= smtExprHasRelevant
	if value {
		r.t.relevanceAdditions++
		record.flags |= smtExprRelevant
	} else if record.flags&smtExprRelevant != 0 {
		r.t.relevanceRemovals++
		record.flags &^= smtExprRelevant
	}
	return marked
}
func (r smtRelevantTable) del(h ast.ASTHandle) {
	if record := r.t.read(h); record != nil {
		if record.flags&smtExprRelevant != 0 {
			r.t.relevanceRemovals++
		}
		record.flags &^= smtExprHasRelevant | smtExprRelevant
	}
}

// smtCacheGenerationTable is context::m_cached_generation (obj_map<expr,
// unsigned>) and m_cache_generation_visited (obj_hashtable<expr>) as two
// flags and a lazily allocated generation column. touched lists the handles
// with either flag set, so reset clears exactly those records.
type smtCacheGenerationTable struct {
	t       *smtExprTable
	touched []ast.ASTHandle
}

func (c *smtCacheGenerationTable) get(h ast.ASTHandle) (uint32, bool) {
	if page := c.t.readPage(h); page != nil && page.records[h.Slot()%smtExprPageSize].flags&smtExprHasCachedGeneration != 0 {
		return page.cached[h.Slot()%smtExprPageSize], true
	}
	return 0, false
}
func (c *smtCacheGenerationTable) record(h ast.ASTHandle) *smtExprRecord {
	record := c.t.write(h)
	if record.flags&(smtExprHasCachedGeneration|smtExprCacheGenerationVisited) == 0 {
		c.touched = append(c.touched, h)
	}
	return record
}
func (c *smtCacheGenerationTable) set(h ast.ASTHandle, generation uint32) {
	record := c.record(h)
	page := c.t.pages[h.Slot()/smtExprPageSize]
	if page.cached == nil {
		page.cached = new([smtExprPageSize]uint32)
	}
	page.cached[h.Slot()%smtExprPageSize] = generation
	record.flags |= smtExprHasCachedGeneration
}

// visit marks h visited and reports whether it already was.
func (c *smtCacheGenerationTable) visit(h ast.ASTHandle) bool {
	if record := c.t.read(h); record != nil && record.flags&smtExprCacheGenerationVisited != 0 {
		return true
	}
	c.record(h).flags |= smtExprCacheGenerationVisited
	return false
}
func (c *smtCacheGenerationTable) reset() {
	for _, h := range c.touched {
		if record := c.t.read(h); record != nil {
			record.flags &^= smtExprHasCachedGeneration | smtExprCacheGenerationVisited
		}
	}
	clear(c.touched)
	if cap(c.touched) > smtCacheGenerationKeep {
		c.touched = nil
	} else {
		c.touched = c.touched[:0]
	}
}
