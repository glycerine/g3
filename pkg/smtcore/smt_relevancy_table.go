package smtcore

import "github.com/glycerine/g3/pkg/ast"

// smtRelevancyTable maps an expression to its handler list, as the
// relevancy propagator's obj_map handler/watch tables do. Lists of one
// manager's expressions are indexed by node slot; every non-empty list is
// referenced by the trail, so its expression (and slot generation) stays
// live while the list is in use. Other handles use the overflow map.
type smtRelevancyTable struct {
	origin  uint64
	entries []smtRelevancyTableEntry
	other   map[ast.ASTHandle][]smtRelevancyHandler
}

type smtRelevancyTableEntry struct {
	key ast.ASTHandle
	hs  []smtRelevancyHandler
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

func (t *smtRelevancyTable) get(h ast.ASTHandle) []smtRelevancyHandler {
	if !t.dense(h) {
		return t.other[h]
	}
	if s := h.Slot(); uint64(s) < uint64(len(t.entries)) && t.entries[s].key == h {
		return t.entries[s].hs
	}
	return nil
}

func (t *smtRelevancyTable) push(h ast.ASTHandle, handler smtRelevancyHandler) {
	if !t.dense(h) {
		if t.other == nil {
			t.other = make(map[ast.ASTHandle][]smtRelevancyHandler)
		}
		t.other[h] = append(t.other[h], handler)
		return
	}
	s := h.Slot()
	if uint64(s) >= uint64(len(t.entries)) {
		n := 2 * len(t.entries)
		if n <= int(s) {
			n = int(s) + 256
		}
		grown := make([]smtRelevancyTableEntry, n)
		copy(grown, t.entries)
		t.entries = grown
	}
	e := &t.entries[s]
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
	var hs []smtRelevancyHandler
	if s := h.Slot(); uint64(s) < uint64(len(t.entries)) && t.entries[s].key == h {
		hs = t.entries[s].hs
		t.entries[s].hs = hs[:len(hs)-1]
		return
	}
	_ = hs[:len(hs)-1] // an absent list panics, as slicing the map's nil list does
}
