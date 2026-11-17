package smtcore

import "github.com/glycerine/g3/pkg/ast"

const smtRelevancyPageSize = 64

// Source: src/smt/smt_relevancy.cpp, relevancy_ehs and its region.
// Each registration prepends one link. Trail undo restores the previous head.
type smtRelevancyTable struct {
	origin uint64
	heads  []int
	other  map[ast.ASTHandle]int

	pages       []*[smtRelevancyPageSize]smtRelevancyLink
	count, free int
}

type smtRelevancyLink struct {
	handler    smtRelevancyHandler
	previous   int
	generation uint32
}

// Index zero is an empty list. The page directory can grow while a
// propagation callback is iterating an earlier list.
func (t *smtRelevancyTable) link(index int) *smtRelevancyLink {
	if index == 0 {
		return nil
	}
	i := index - 1
	return &t.pages[i/smtRelevancyPageSize][i%smtRelevancyPageSize]
}

func (t *smtRelevancyTable) head(h ast.ASTHandle) int {
	if h.Origin() != t.origin || h.IsNull() {
		return t.other[h]
	}
	s := h.Slot()
	if uint64(s) >= uint64(len(t.heads)) {
		return 0
	}
	head := t.heads[s]
	if head != 0 && t.link(head).generation != h.Generation() {
		return 0
	}
	return head
}

func (t *smtRelevancyTable) push(h ast.ASTHandle, handler smtRelevancyHandler) {
	if t.origin == 0 && !h.IsNull() {
		t.origin = h.Origin()
	}
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
	if h.Origin() != t.origin || h.IsNull() {
		if t.other == nil {
			t.other = make(map[ast.ASTHandle]int)
		}
		t.other[h] = index
		return
	}
	s := int(h.Slot())
	if s >= len(t.heads) {
		t.heads = append(t.heads, make([]int, s+1-len(t.heads))...)
	}
	t.heads[s] = index
}

func (t *smtRelevancyTable) pop(h ast.ASTHandle) {
	index := t.head(h)
	link := t.link(index)
	if h.Origin() == t.origin && !h.IsNull() {
		t.heads[h.Slot()] = link.previous
	} else if link.previous == 0 {
		delete(t.other, h)
	} else {
		t.other[h] = link.previous
	}
	// Clear any slice of target expressions when its scope is popped.
	*link = smtRelevancyLink{previous: t.free}
	t.free = index
}
