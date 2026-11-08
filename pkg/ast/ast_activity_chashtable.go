package ast

// Source: src/util/chashtable.h, concretely for act_cache's
// cmap<expr*, expr*, obj_ptr_hash<expr>> table. act_cache::dec_refs walks
// that table, so the order in which a rewriter cache releases its keys and
// values (and so the order in which their ids are recycled) is the chained
// table's slot-and-chain order on the AST hash. Lookups stay in the
// ActivityCache's dictionary; this table only reproduces the layout.

const (
	activityChashInitSlots  = 8 // chashtable::default_init_slots
	activityChashInitCellar = 2 // chashtable::default_init_cellar
	activityChashNil        = -1
	activityChashFree       = -2 // cell::m_next == reinterpret_cast<cell*>(1)
)

type activityChashCell struct {
	next  int32
	entry *astActivityEntry
}

type activityChashtable struct {
	cells      []activityChashCell
	slots      int32
	usedSlots  int32
	size       int32
	nextCell   int32
	freeCell   int32
	initSlots  int32
	initCellar int32
}

func newActivityChashtable() activityChashtable {
	t := activityChashtable{initSlots: activityChashInitSlots, initCellar: activityChashInitCellar}
	t.init(t.initSlots, t.initCellar)
	return t
}

func activityChashAlloc(n int32) []activityChashCell {
	cells := make([]activityChashCell, n)
	for i := range cells {
		cells[i].next = activityChashFree
	}
	return cells
}

func (t *activityChashtable) init(slots, cellar int32) {
	t.cells = activityChashAlloc(slots + cellar)
	t.slots = slots
	t.usedSlots = 0
	t.size = 0
	t.nextCell = slots
	t.freeCell = activityChashNil
}

func (t *activityChashtable) hasFreeCells() bool {
	return t.freeCell != activityChashNil || t.nextCell < int32(len(t.cells))
}

func (t *activityChashtable) getFreeCell() int32 {
	if t.freeCell != activityChashNil {
		c := t.freeCell
		t.freeCell = t.cells[c].next
		return c
	}
	c := t.nextCell
	t.nextCell++
	return c
}

func (t *activityChashtable) recycleCell(c int32) {
	t.cells[c].next = t.freeCell
	t.freeCell = c
}

// copyTable returns the next free cellar cell, or -1 when the cellar is too
// small for the target layout.
func (t *activityChashtable) copyTable(target []activityChashCell, targetSlots int32) (int32, int32) {
	mask := uint32(targetSlots - 1)
	used := int32(0)
	cellar := targetSlots
	end := int32(len(target))
	for i := int32(0); i < t.slots; i++ {
		if t.cells[i].next == activityChashFree {
			continue
		}
		for it := i; it != activityChashNil; it = t.cells[it].next {
			entry := t.cells[it].entry
			idx := int32(entry.hash & mask)
			if target[idx].next == activityChashFree {
				target[idx] = activityChashCell{next: activityChashNil, entry: entry}
				used++
				continue
			}
			if cellar == end {
				return -1, 0
			}
			target[cellar] = target[idx]
			target[idx] = activityChashCell{next: cellar, entry: entry}
			cellar++
		}
	}
	return cellar, used
}

func (t *activityChashtable) expandTable() {
	currCellar := int32(len(t.cells)) - t.slots
	newSlots := t.slots * 2
	newCellar := currCellar * 2
	for {
		target := activityChashAlloc(newSlots + newCellar)
		next, used := t.copyTable(target, newSlots)
		if next >= 0 {
			t.cells = target
			t.slots = newSlots
			t.usedSlots = used
			t.nextCell = next
			t.freeCell = activityChashNil
			return
		}
		newCellar *= 2
	}
}

// insert is insert_if_not_there for a key known to be absent: a colliding
// entry becomes the chain head and the old head moves to a cellar cell.
func (t *activityChashtable) insert(entry *astActivityEntry) {
	if !t.hasFreeCells() {
		t.expandTable()
	}
	idx := int32(entry.hash & uint32(t.slots-1))
	if t.cells[idx].next == activityChashFree {
		t.size++
		t.usedSlots++
		t.cells[idx] = activityChashCell{next: activityChashNil, entry: entry}
		return
	}
	t.size++
	c := t.getFreeCell()
	t.cells[c] = t.cells[idx]
	t.cells[idx] = activityChashCell{next: c, entry: entry}
}

func (t *activityChashtable) erase(entry *astActivityEntry) {
	c := int32(entry.hash & uint32(t.slots-1))
	if t.cells[c].next == activityChashFree {
		return
	}
	prev := int32(activityChashNil)
	for c != activityChashNil {
		if t.cells[c].entry == entry {
			t.size--
			if prev == activityChashNil {
				next := t.cells[c].next
				if next == activityChashNil {
					t.usedSlots--
					t.cells[c] = activityChashCell{next: activityChashFree}
				} else {
					t.cells[c] = t.cells[next]
					t.recycleCell(next)
				}
			} else {
				t.cells[prev].next = t.cells[c].next
				t.recycleCell(c)
			}
			return
		}
		prev = c
		c = t.cells[c].next
	}
}

// each visits the entries in chashtable::iterator order.
func (t *activityChashtable) each(visit func(*astActivityEntry) error) error {
	for i := int32(0); i < t.slots; i++ {
		if t.cells[i].next == activityChashFree {
			continue
		}
		for it := i; it != activityChashNil; it = t.cells[it].next {
			if e := visit(t.cells[it].entry); e != nil {
				return e
			}
		}
	}
	return nil
}

// reset is chashtable::reset: a non-empty table is finalized.
func (t *activityChashtable) reset() {
	if t.size == 0 {
		return
	}
	t.finalize()
}

func (t *activityChashtable) finalize() {
	t.init(t.initSlots, t.initCellar)
}
