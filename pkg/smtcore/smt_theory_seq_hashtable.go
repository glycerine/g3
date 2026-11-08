package smtcore

// Source: src/util/hashtable.h core_hashtable and src/util/obj_hashtable.h
// obj_hash_entry, concretely for obj_hashtable<expr>. theory_seq iterates
// m_length and m_fixed, so their traversal order must be the source's:
// open addressing on the AST hash, with deleted markers and the same growth
// and cleanup thresholds.

import "github.com/glycerine/g3/pkg/ast"

const (
	seqHashtableInitialCapacity = 8  // DEFAULT_HASHTABLE_INITIAL_CAPACITY
	seqHashtableSmallCapacity   = 64 // SMALL_TABLE_CAPACITY
)

type seqObjEntryState uint8

const (
	seqObjFree seqObjEntryState = iota
	seqObjDeleted
	seqObjUsed
)

type seqObjEntry struct {
	state seqObjEntryState
	e     ast.ASTHandle
	hash  uint32
}

// seqObjHashtable is obj_hashtable<expr>.
type seqObjHashtable struct {
	manager    *ast.ASTManager
	table      []seqObjEntry
	size       int
	numDeleted int
}

func newSeqObjHashtable(m *ast.ASTManager) seqObjHashtable {
	return seqObjHashtable{manager: m, table: make([]seqObjEntry, seqHashtableInitialCapacity)}
}

func (h *seqObjHashtable) hashOf(e ast.ASTHandle) (uint32, error) { return h.manager.GetHash(e) }

func seqObjMoveTable(source, target []seqObjEntry) {
	mask := uint32(len(target) - 1)
	for _, entry := range source {
		if entry.state != seqObjUsed {
			continue
		}
		begin := int(entry.hash & mask)
		placed := false
		for i := begin; i < len(target); i++ {
			if target[i].state == seqObjFree {
				target[i] = entry
				placed = true
				break
			}
		}
		for i := 0; !placed && i < begin; i++ {
			if target[i].state == seqObjFree {
				target[i] = entry
				placed = true
			}
		}
	}
}

func (h *seqObjHashtable) expandTable() {
	newTable := make([]seqObjEntry, len(h.table)<<1)
	seqObjMoveTable(h.table, newTable)
	h.table = newTable
	h.numDeleted = 0
}

func (h *seqObjHashtable) removeDeletedEntries() {
	newTable := make([]seqObjEntry, len(h.table))
	seqObjMoveTable(h.table, newTable)
	h.table = newTable
	h.numDeleted = 0
}

func (h *seqObjHashtable) insert(e ast.ASTHandle) error {
	if (h.size+h.numDeleted)<<2 > len(h.table)*3 {
		h.expandTable()
	}
	hash, err := h.hashOf(e)
	if err != nil {
		return err
	}
	mask := uint32(len(h.table) - 1)
	begin := int(hash & mask)
	delEntry := -1
	visit := func(i int) bool {
		curr := &h.table[i]
		switch curr.state {
		case seqObjUsed:
			if curr.hash == hash && curr.e == e {
				return true
			}
		case seqObjFree:
			n := i
			if delEntry >= 0 {
				n = delEntry
				h.numDeleted--
			}
			h.table[n] = seqObjEntry{state: seqObjUsed, e: e, hash: hash}
			h.size++
			return true
		default:
			delEntry = i
		}
		return false
	}
	for i := begin; i < len(h.table); i++ {
		if visit(i) {
			return nil
		}
	}
	for i := 0; i < begin; i++ {
		if visit(i) {
			return nil
		}
	}
	return ast.ErrASTDomain // UNREACHABLE
}

func (h *seqObjHashtable) find(e ast.ASTHandle) (int, error) {
	hash, err := h.hashOf(e)
	if err != nil {
		return -1, err
	}
	mask := uint32(len(h.table) - 1)
	begin := int(hash & mask)
	for _, r := range [2][2]int{{begin, len(h.table)}, {0, begin}} {
		for i := r[0]; i < r[1]; i++ {
			curr := &h.table[i]
			if curr.state == seqObjUsed {
				if curr.hash == hash && curr.e == e {
					return i, nil
				}
			} else if curr.state == seqObjFree {
				return -1, nil
			}
		}
	}
	return -1, nil
}

func (h *seqObjHashtable) contains(e ast.ASTHandle) (bool, error) {
	i, err := h.find(e)
	return i >= 0, err
}

func (h *seqObjHashtable) remove(e ast.ASTHandle) error {
	i, err := h.find(e)
	if err != nil || i < 0 {
		return err
	}
	next := i + 1
	if next == len(h.table) {
		next = 0
	}
	if h.table[next].state == seqObjFree {
		h.table[i] = seqObjEntry{}
		h.size--
		return nil
	}
	h.table[i] = seqObjEntry{state: seqObjDeleted}
	h.numDeleted++
	h.size--
	if h.numDeleted > h.size && h.numDeleted > seqHashtableSmallCapacity {
		h.removeDeletedEntries()
	}
	return nil
}

// elems returns the elements in the source's iteration order.
func (h *seqObjHashtable) elems() []ast.ASTHandle {
	result := make([]ast.ASTHandle, 0, h.size)
	for _, entry := range h.table {
		if entry.state == seqObjUsed {
			result = append(result, entry.e)
		}
	}
	return result
}
