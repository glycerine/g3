package smtcore

// Source: context::m_expr2bool_var, m_app2enode, and m_relevancy's mark, which
// are vectors indexed by expression id. Here one record per AST slot holds all
// of an expression's solver state, so a lookup is an index and a generation test.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

const (
	smtExprHasBoolVar uint8 = 1 << iota
	smtExprHasAssignment
	smtExprHasRelevant
	smtExprRelevant
)

type smtExprRecord struct {
	enode      *SMTEnode
	generation uint32
	boolVar    int32
	assignment lbool.LBool
	flags      uint8
}

// smtExprTable is owned by one SMTCheckerContext and borrows nothing. A record
// whose generation differs from a handle's describes a retired expression.
type smtExprTable struct {
	records      []smtExprRecord
	origin       uint64
	enodeCount   int
	boolVarCount int
}

func newSMTExprTable() *smtExprTable { return &smtExprTable{} }

// read returns the live record for a handle, or nil.
func (t *smtExprTable) read(h ast.ASTHandle) *smtExprRecord {
	slot := h.Slot()
	if uint64(slot) >= uint64(len(t.records)) || h.Origin() != t.origin {
		return nil
	}
	record := &t.records[slot]
	if record.generation != h.Generation() {
		return nil
	}
	return record
}

// write returns the record for a handle, retiring any older generation's state.
func (t *smtExprTable) write(h ast.ASTHandle) *smtExprRecord {
	if t.origin == 0 {
		t.origin = h.Origin()
	} else if h.Origin() != t.origin {
		panic("smtcore: expression table used with a foreign manager's handle")
	}
	slot := int(h.Slot())
	if slot >= len(t.records) {
		t.records = append(t.records, make([]smtExprRecord, slot+1-len(t.records))...)
	}
	record := &t.records[slot]
	if record.generation != h.Generation() {
		if record.enode != nil {
			t.enodeCount--
		}
		if record.flags&smtExprHasBoolVar != 0 {
			t.boolVarCount--
		}
		*record = smtExprRecord{generation: h.Generation()}
	}
	return record
}

type smtEnodeTable struct{ t *smtExprTable }

func (e smtEnodeTable) get(h ast.ASTHandle) *SMTEnode {
	if record := e.t.read(h); record != nil {
		return record.enode
	}
	return nil
}
func (e smtEnodeTable) lookup(h ast.ASTHandle) (*SMTEnode, bool) {
	node := e.get(h)
	return node, node != nil
}
func (e smtEnodeTable) set(h ast.ASTHandle, node *SMTEnode) {
	record := e.t.write(h)
	if record.enode == nil && node != nil {
		e.t.enodeCount++
	} else if record.enode != nil && node == nil {
		e.t.enodeCount--
	}
	record.enode = node
}
func (e smtEnodeTable) del(h ast.ASTHandle) {
	if record := e.t.read(h); record != nil && record.enode != nil {
		record.enode = nil
		e.t.enodeCount--
	}
}
func (e smtEnodeTable) count() int { return e.t.enodeCount }

// all lists live enodes in slot order, which is deterministic.
func (e smtEnodeTable) all() []*SMTEnode {
	result := make([]*SMTEnode, 0, e.t.enodeCount)
	for i := range e.t.records {
		if node := e.t.records[i].enode; node != nil {
			result = append(result, node)
		}
	}
	return result
}

type smtBoolVarTable struct{ t *smtExprTable }

func (b smtBoolVarTable) lookup(h ast.ASTHandle) (int32, bool) {
	if record := b.t.read(h); record != nil && record.flags&smtExprHasBoolVar != 0 {
		return record.boolVar, true
	}
	return 0, false
}
func (b smtBoolVarTable) get(h ast.ASTHandle) int32 {
	variable, _ := b.lookup(h)
	return variable
}
func (b smtBoolVarTable) set(h ast.ASTHandle, variable int32) {
	record := b.t.write(h)
	if record.flags&smtExprHasBoolVar == 0 {
		b.t.boolVarCount++
	}
	record.boolVar = variable
	record.flags |= smtExprHasBoolVar
}
func (b smtBoolVarTable) del(h ast.ASTHandle) {
	if record := b.t.read(h); record != nil && record.flags&smtExprHasBoolVar != 0 {
		record.flags &^= smtExprHasBoolVar
		record.boolVar = 0
		b.t.boolVarCount--
	}
}
func (b smtBoolVarTable) count() int { return b.t.boolVarCount }

type smtAssignmentTable struct{ t *smtExprTable }

func (a smtAssignmentTable) lookup(h ast.ASTHandle) (lbool.LBool, bool) {
	if record := a.t.read(h); record != nil && record.flags&smtExprHasAssignment != 0 {
		return record.assignment, true
	}
	return 0, false
}
func (a smtAssignmentTable) get(h ast.ASTHandle) lbool.LBool {
	value, _ := a.lookup(h)
	return value
}
func (a smtAssignmentTable) set(h ast.ASTHandle, value lbool.LBool) {
	record := a.t.write(h)
	record.assignment = value
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
func (r smtRelevantTable) set(h ast.ASTHandle, value bool) {
	record := r.t.write(h)
	record.flags |= smtExprHasRelevant
	if value {
		record.flags |= smtExprRelevant
	} else {
		record.flags &^= smtExprRelevant
	}
}
func (r smtRelevantTable) del(h ast.ASTHandle) {
	if record := r.t.read(h); record != nil {
		record.flags &^= smtExprHasRelevant | smtExprRelevant
	}
}
