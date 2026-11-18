package smtcore

// Source: src/smt/smt_relevancy.cpp: relevancy_ehs (a list<relevancy_eh*>
// allocated in the context region) and the obj_map handler/watch tables.

import "github.com/glycerine/g3/pkg/ast"

const smtRelevancyPageSize = 256

// smtRelevancyTable maps an expression to its handler list, as the
// relevancy propagator's obj_map handler/watch tables do. A registration
// prepends one link (a cons cell of relevancy_ehs) holding a reference into
// the propagator's handler pool (ehs); undoing it restores the previous
// head. Links live in fixed pages and popped links are recycled, as the
// region reuses its memory after a scope pop. Heads of one manager's
// expressions are indexed by node slot (4 bytes per slot); every non-empty
// list is referenced by the trail, so its expression (and slot generation)
// stays live while the list is in use. Other handles use the overflow map.
type smtRelevancyTable struct {
	origin uint64
	// heads[s] is 1 + the link index of slot s's newest registration, or 0.
	heads []uint32
	other map[ast.ASTHandle]uint32

	pages       []*[smtRelevancyPageSize]smtRelevancyLink
	count, free uint32
}

// smtRelevancyLink is one list cell: the handler reference (1 + its ehs
// index), the next older cell, and the generation of the slot's key.
type smtRelevancyLink struct {
	handler, previous, generation uint32
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

// link returns cell index (1-based). The page directory can grow while a
// propagation callback walks an earlier list; cells never move.
func (t *smtRelevancyTable) link(index uint32) *smtRelevancyLink {
	i := index - 1
	return &t.pages[i/smtRelevancyPageSize][i%smtRelevancyPageSize]
}

// head returns h's newest cell, or 0 for an empty list.
func (t *smtRelevancyTable) head(h ast.ASTHandle) uint32 {
	if !t.dense(h) {
		return t.other[h]
	}
	s := h.Slot()
	if uint64(s) >= uint64(len(t.heads)) {
		return 0
	}
	head := t.heads[s]
	if head != 0 && t.link(head).generation != h.Generation() {
		// The slot's previous expression is dead, so its list is empty.
		return 0
	}
	return head
}

func (t *smtRelevancyTable) push(h ast.ASTHandle, handler uint32) {
	previous := t.head(h)
	index := t.free
	if index == 0 {
		if t.count%smtRelevancyPageSize == 0 {
			t.pages = append(t.pages, new([smtRelevancyPageSize]smtRelevancyLink))
		}
		t.count++
		index = t.count
	} else {
		t.free = t.link(index).previous
	}
	*t.link(index) = smtRelevancyLink{handler: handler, previous: previous, generation: h.Generation()}
	if !t.dense(h) {
		if t.other == nil {
			t.other = make(map[ast.ASTHandle]uint32)
		}
		t.other[h] = index
		return
	}
	s := h.Slot()
	if uint64(s) >= uint64(len(t.heads)) {
		n := 2 * len(t.heads)
		if n <= int(s) {
			n = int(s) + 256
		}
		grown := make([]uint32, n)
		copy(grown, t.heads)
		t.heads = grown
	}
	t.heads[s] = index
}

// pop drops the newest handler of h's list and recycles its cell.
func (t *smtRelevancyTable) pop(h ast.ASTHandle) {
	index := t.head(h)
	cell := t.link(index) // an absent list panics (index 0)
	if t.dense(h) {
		t.heads[h.Slot()] = cell.previous
	} else if cell.previous == 0 {
		delete(t.other, h)
	} else {
		t.other[h] = cell.previous
	}
	*cell = smtRelevancyLink{previous: t.free}
	t.free = index
}
