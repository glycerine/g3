package ackermannization

// Source: src/util/hashtable.h core_hashtable and src/util/obj_hashtable.h,
// concretely for ackr_helper's app_set (obj_hashtable<app>) and fun2terms_map
// (obj_map<func_decl, app_set*>). lackr iterates both, and that order picks
// the abstraction constants and the order of the Ackermann lemmas, so it
// must be the source's: open addressing on the AST hash. Entries are only
// inserted here, never removed.

import "github.com/glycerine/g3/pkg/ast"

const ackrHashtableInitialCapacity = 8 // DEFAULT_HASHTABLE_INITIAL_CAPACITY

type ackrObjEntry struct {
	used  bool
	key   ast.ASTHandle
	hash  uint32
	value int // obj_map value: index of the decl's app_set in termGroups.sets
}

type ackrObjHashtable struct {
	table []ackrObjEntry
	size  int
}

func newAckrObjHashtable() *ackrObjHashtable {
	return &ackrObjHashtable{table: make([]ackrObjEntry, ackrHashtableInitialCapacity)}
}

func ackrMoveTable(source, target []ackrObjEntry) {
	mask := uint32(len(target) - 1)
	for _, entry := range source {
		if !entry.used {
			continue
		}
		begin := int(entry.hash & mask)
		placed := false
		for i := begin; i < len(target) && !placed; i++ {
			if !target[i].used {
				target[i] = entry
				placed = true
			}
		}
		for i := 0; i < begin && !placed; i++ {
			if !target[i].used {
				target[i] = entry
				placed = true
			}
		}
	}
}

// insert adds key (core_hashtable::insert) and returns its slot.
func (h *ackrObjHashtable) insert(m *ast.ASTManager, key ast.ASTHandle) (int, bool, error) {
	if h.size<<2 > len(h.table)*3 {
		bigger := make([]ackrObjEntry, len(h.table)<<1)
		ackrMoveTable(h.table, bigger)
		h.table = bigger
	}
	hash, err := m.GetHash(key)
	if err != nil {
		return -1, false, err
	}
	mask := uint32(len(h.table) - 1)
	begin := int(hash & mask)
	for _, r := range [2][2]int{{begin, len(h.table)}, {0, begin}} {
		for i := r[0]; i < r[1]; i++ {
			curr := &h.table[i]
			if curr.used {
				if curr.hash == hash && curr.key == key {
					return i, false, nil
				}
				continue
			}
			*curr = ackrObjEntry{used: true, key: key, hash: hash}
			h.size++
			return i, true, nil
		}
	}
	return -1, false, ast.ErrASTDomain // UNREACHABLE
}

// entries returns the used entries in table (iteration) order.
func (h *ackrObjHashtable) entries() []ackrObjEntry {
	out := make([]ackrObjEntry, 0, h.size)
	for _, entry := range h.table {
		if entry.used {
			out = append(out, entry)
		}
	}
	return out
}

// keys returns the keys in table (iteration) order.
func (h *ackrObjHashtable) keys() []ast.ASTHandle {
	out := make([]ast.ASTHandle, 0, h.size)
	for _, entry := range h.table {
		if entry.used {
			out = append(out, entry.key)
		}
	}
	return out
}
