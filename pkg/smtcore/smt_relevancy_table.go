package smtcore

import "github.com/glycerine/g3/pkg/ast"

// smtRelevancyTable maps an expression to its handler list, as the
// relevancy propagator's obj_map handler/watch tables do. Lists of one
// manager's expressions are found through a slot-indexed table of list
// numbers (4 bytes per node slot; an obj_map holds only the watched
// expressions, and most slots have no list); every non-empty list is
// referenced by the trail, so its expression (and slot generation) stays
// live while the list is in use. Other handles use the overflow map.
type smtRelevancyTable struct {
	origin uint64
	// index[s] is 1 + the position in lists of slot s's list, or 0.
	index []uint32
	lists []smtRelevancyTableEntry
	other map[ast.ASTHandle][]uint32
}

// hs holds references into the propagator's ehs.
type smtRelevancyTableEntry struct {
	key ast.ASTHandle
	hs  []uint32
}

func (t *smtRelevancyTable) dense(h ast.ASTHandle) bool {
	if h.IsNull() {
		return false
	}
	if t.origin == 0 {
		t.origin = h.Origin()
	}
	return h.Origin() == t.origin
}

// entry returns slot s's list record, or nil.
func (t *smtRelevancyTable) entry(s uint32) *smtRelevancyTableEntry {
	if uint64(s) >= uint64(len(t.index)) {
		return nil
	}
	if i := t.index[s]; i != 0 {
		return &t.lists[i-1]
	}
	return nil
}

func (t *smtRelevancyTable) get(h ast.ASTHandle) []uint32 {
	if !t.dense(h) {
		return t.other[h]
	}
	if e := t.entry(h.Slot()); e != nil && e.key == h && len(e.hs) != 0 {
		return e.hs
	}
	return nil
}

func (t *smtRelevancyTable) push(h ast.ASTHandle, handler uint32) {
	if !t.dense(h) {
		if t.other == nil {
			t.other = make(map[ast.ASTHandle][]uint32)
		}
		t.other[h] = append(t.other[h], handler)
		return
	}
	s := h.Slot()
	if uint64(s) >= uint64(len(t.index)) {
		n := 2 * len(t.index)
		if n <= int(s) {
			n = int(s) + 256
		}
		grown := make([]uint32, n)
		copy(grown, t.index)
		t.index = grown
	}
	if t.index[s] == 0 {
		t.lists = append(t.lists, smtRelevancyTableEntry{})
		t.index[s] = uint32(len(t.lists))
	}
	e := &t.lists[t.index[s]-1]
	if e.key != h {
		// The slot's previous expression is dead, so its list is empty.
		e.key = h
		e.hs = e.hs[:0]
	}
	e.hs = append(e.hs, handler)
}

// pop drops the newest handler of h's list.
func (t *smtRelevancyTable) pop(h ast.ASTHandle) {
	if !t.dense(h) {
		hs := t.other[h]
		t.other[h] = hs[:len(hs)-1]
		return
	}
	var hs []uint32
	if e := t.entry(h.Slot()); e != nil && e.key == h {
		hs = e.hs
		e.hs = hs[:len(hs)-1]
		return
	}
	_ = hs[:len(hs)-1] // an absent list panics, as slicing the map's nil list does
}
