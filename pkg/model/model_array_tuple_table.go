package model

// Native tuple lookup for src/model/model_evaluator.cpp:337-416.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/hash"
)

// arrayTupleTable is evaluator_cfg::args_table, a
// hashtable<expr*const*, args_hash, args_eq> (src/util/hashtable.h
// core_hashtable). mk_array_eq_core iterates table1, so the conjuncts follow
// its table order: open addressing on args_hash, which is get_composite_hash
// with default_kind_hash_proc (17) over the AST hashes of the first arity
// elements; args_eq compares those elements by identity. Rows borrow the
// owning expansion slices; the table holds no references.
type arrayTupleTable struct {
	manager    *ast.ASTManager
	arity      int
	table      []arrayTupleEntry
	size       int
	numDeleted int
}

type arrayTupleEntryState uint8

const (
	arrayTupleFree arrayTupleEntryState = iota
	arrayTupleDeleted
	arrayTupleUsed
)

type arrayTupleEntry struct {
	row   []ast.ASTHandle
	hash  uint32
	state arrayTupleEntryState
}

const (
	arrayTupleInitialCapacity = 8  // DEFAULT_HASHTABLE_INITIAL_CAPACITY
	arrayTupleSmallCapacity   = 64 // SMALL_TABLE_CAPACITY
)

// hashRow is args_hash::operator()(expr* const*).
func (t *arrayTupleTable) hashRow(row []ast.ASTHandle) (uint32, error) {
	if t.manager == nil || t.arity <= 0 || len(row) <= t.arity {
		return 0, ast.ErrASTDomain
	}
	var failure error
	h := hash.Composite(17, uint32(t.arity), func(i uint32) uint32 {
		v, e := t.manager.GetHash(row[i])
		if e != nil && failure == nil {
			failure = e
		}
		return v
	})
	return h, failure
}

// equals is args_eq.
func (t *arrayTupleTable) equals(a, b []ast.ASTHandle) bool {
	for i := 0; i < t.arity; i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (t *arrayTupleTable) ensureTable() {
	if t.table == nil {
		t.table = make([]arrayTupleEntry, arrayTupleInitialCapacity)
	}
}

// moveTable is core_hashtable::move_table.
func arrayTupleMoveTable(source, target []arrayTupleEntry) {
	mask := uint32(len(target) - 1)
	for _, entry := range source {
		if entry.state != arrayTupleUsed {
			continue
		}
		begin := int(entry.hash & mask)
		placed := false
		for i := begin; i < len(target) && !placed; i++ {
			if target[i].state == arrayTupleFree {
				target[i], placed = entry, true
			}
		}
		for i := 0; i < begin && !placed; i++ {
			if target[i].state == arrayTupleFree {
				target[i], placed = entry, true
			}
		}
	}
}

// findIndex is core_hashtable::find_core; it returns -1 when absent.
func (t *arrayTupleTable) findIndex(row []ast.ASTHandle) (int, error) {
	h, e := t.hashRow(row)
	if e != nil {
		return -1, e
	}
	t.ensureTable()
	mask := len(t.table) - 1
	begin := int(h) & mask
	for k := 0; k < len(t.table); k++ {
		idx := (begin + k) & mask
		curr := &t.table[idx]
		switch curr.state {
		case arrayTupleUsed:
			if curr.hash == h && t.equals(curr.row, row) {
				return idx, nil
			}
		case arrayTupleFree:
			return -1, nil
		}
	}
	return -1, nil
}

func (t *arrayTupleTable) find(row []ast.ASTHandle) ([]ast.ASTHandle, bool, error) {
	idx, e := t.findIndex(row)
	if e != nil || idx < 0 {
		return nil, false, e
	}
	return t.table[idx].row, true, nil
}

// insert is core_hashtable::insert: an equal entry has its data replaced.
func (t *arrayTupleTable) insert(row []ast.ASTHandle) error {
	h, e := t.hashRow(row)
	if e != nil {
		return e
	}
	t.ensureTable()
	if (t.size+t.numDeleted)<<2 > len(t.table)*3 {
		target := make([]arrayTupleEntry, len(t.table)<<1)
		arrayTupleMoveTable(t.table, target)
		t.table = target
		t.numDeleted = 0
	}
	mask := len(t.table) - 1
	begin := int(h) & mask
	del := -1
	for k := 0; k < len(t.table); k++ {
		idx := (begin + k) & mask
		curr := &t.table[idx]
		switch curr.state {
		case arrayTupleUsed:
			if curr.hash == h && t.equals(curr.row, row) {
				curr.row = row
				return nil
			}
		case arrayTupleFree:
			target := idx
			if del >= 0 {
				target = del
				t.numDeleted--
			}
			t.table[target] = arrayTupleEntry{row: row, hash: h, state: arrayTupleUsed}
			t.size++
			return nil
		default:
			del = idx
		}
	}
	return ast.ErrASTDomain
}

// remove is core_hashtable::remove.
func (t *arrayTupleTable) remove(row []ast.ASTHandle) error {
	idx, e := t.findIndex(row)
	if e != nil || idx < 0 {
		return e
	}
	next := idx + 1
	if next == len(t.table) {
		next = 0
	}
	if t.table[next].state == arrayTupleFree {
		t.table[idx] = arrayTupleEntry{}
		t.size--
		return nil
	}
	t.table[idx] = arrayTupleEntry{state: arrayTupleDeleted}
	t.numDeleted++
	t.size--
	if t.numDeleted > t.size && t.numDeleted > arrayTupleSmallCapacity {
		// remove_deleted_entries
		target := make([]arrayTupleEntry, len(t.table))
		arrayTupleMoveTable(t.table, target)
		t.table = target
		t.numDeleted = 0
	}
	return nil
}

// entries lists the live rows in table (iteration) order.
func (t *arrayTupleTable) entries() [][]ast.ASTHandle {
	result := make([][]ast.ASTHandle, 0, t.size)
	for _, entry := range t.table {
		if entry.state == arrayTupleUsed {
			result = append(result, entry.row)
		}
	}
	return result
}

func (t *arrayTupleTable) close() { *t = arrayTupleTable{} }
