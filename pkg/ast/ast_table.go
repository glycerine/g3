package ast

// Source: src/ast/ast.h:920-924; src/ast/ast.cpp:1483-1497, 1714-1780;
// src/util/insdict.h of the C++ fork. FORK FIX (owner decision 2026-11-18,
// see FORK_BUGFIXES.md): the fork's ast_table is an insertion-ordered
// insdict, the C++ port of github.com/glycerine/insdict v0.14.6, instead of
// chashtable. This table keeps insdict's exact structure, so iteration
// (teardown free order) and capacity (the (ast-table ...) verbose lines)
// agree with the fork.
//
// Canonical entries borrow exact node tokens. entries lists them by
// insertion; an erased entry leaves a hole (a zero ref) that only a rebuild
// drops. indices is an open-addressed index of entry positions with one
// control byte per slot in tags, probed with CPython's perturbed sequence.
// A new entry is refused room once len(entries), holes included, reaches
// 2/3 of the index; the index is then rebuilt to astTableSizeFor(size)
// slots, compacting holes in order. Capacity and order thus follow from
// the insert/erase sequence alone, as in the fork.

import (
	"strconv"

	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/insdict"
)

type astCanonicalTable struct {
	indices []int32 // astSlotEmpty, astSlotDummy, or a position in entries
	tags    []byte  // astTagEmpty, astTagDummy, or astTagUsed | hash>>57
	entries []astTableEntry
	mask    uint64
	size    uint32 // live entries
}

type astTableEntry struct {
	hash uint64 // insdict.Mix64 of the node hash
	ref  astNodeRef
}

const (
	astSlotEmpty     int32 = -1
	astSlotDummy     int32 = -2
	astTableMinSlots       = 8
	astTagEmpty      byte  = 0
	astTagDummy      byte  = 1
	astTagUsed       byte  = 0x80
)

// astTableHash is the fork's ast_insdict_hash: insdict_mix64(n->hash()).
func astTableHash(hash uint32) uint64 { return insdict.Mix64(uint64(hash)) }

func astTableTag(h uint64) byte { return astTagUsed | byte(h>>57) }

// astTableSizeFor is insdict sizeFor: the smallest power of two >= 8 and
// >= 3*live.
func astTableSizeFor(live uint32) int {
	size := astTableMinSlots
	for size < int(live)*3 {
		size <<= 1
	}
	return size
}

// capacity is ast_table::capacity(): the index length (0 when empty).
func (t *astCanonicalTable) capacity() uint32 { return uint32(len(t.indices)) }

func (t *astCanonicalTable) usable() int { return len(t.indices) * 2 / 3 }

// freeSlot is the first empty-or-dummy slot on h's probe path.
func (t *astCanonicalTable) freeSlot(h uint64) uint64 {
	i, perturb := h&t.mask, h
	for t.tags[i] >= astTagUsed {
		perturb >>= 5
		i = (i*5 + perturb + 1) & t.mask
	}
	return i
}

// rebuild resizes the index to size slots, dropping holes and keeping
// insertion order.
func (t *astCanonicalTable) rebuild(size int) {
	if size == len(t.indices) {
		clear(t.tags)
	} else {
		t.indices = make([]int32, size)
		t.tags = make([]byte, size)
	}
	for i := range t.indices {
		t.indices[i] = astSlotEmpty
	}
	t.mask = uint64(size - 1)
	old := t.entries
	capacity := size * 2 / 3
	if cap(old) >= capacity {
		t.entries = old[:0]
	} else {
		t.entries = make([]astTableEntry, 0, capacity)
	}
	for _, e := range old {
		if e.ref.slot == 0 {
			continue
		}
		slot := t.freeSlot(e.hash)
		t.indices[slot] = int32(len(t.entries))
		t.tags[slot] = astTableTag(e.hash)
		t.entries = append(t.entries, e)
	}
	if len(t.entries) < len(old) && cap(old) >= capacity {
		clear(old[len(t.entries):])
	}
}

// add appends a key known to be absent (insdict Put of a new key); slot is
// the empty slot where the failed lookup ended, valid only while the table
// has no holes and no rebuild is due.
func (t *astCanonicalTable) add(h uint64, ref astNodeRef, slot uint64, haveSlot bool) {
	if t.indices == nil {
		t.rebuild(astTableMinSlots)
		haveSlot = false
	}
	if len(t.entries) >= t.usable() {
		t.rebuild(astTableSizeFor(t.size))
		haveSlot = false
	}
	if !haveSlot || int(t.size) != len(t.entries) {
		slot = t.freeSlot(h)
	}
	t.indices[slot] = int32(len(t.entries))
	t.tags[slot] = astTableTag(h)
	t.entries = append(t.entries, astTableEntry{hash: h, ref: ref})
	t.size++
}

// tableEntries lists the canonical table's entries in insertion order.
func (m *ASTManager) tableEntries() []astNodeRef {
	t := &m.table
	out := make([]astNodeRef, 0, t.size)
	for _, e := range t.entries {
		if e.ref.slot != 0 {
			out = append(out, e.ref)
		}
	}
	return out
}

func (m *ASTManager) initTable() {
	m.table = astCanonicalTable{}
}

// tableLookup finds the entry structurally equal to key, or reports the
// empty slot where the probe ended.
func (m *ASTManager) tableLookup(key astNodeRef, h uint64) (astNodeRef, bool, uint64, error) {
	t := &m.table
	tag := astTableTag(h)
	i, perturb := h&t.mask, h
	for {
		ctrl := t.tags[i]
		if ctrl == astTagEmpty {
			return astNodeRef{}, false, i, nil
		}
		if ctrl == tag {
			e := &t.entries[t.indices[i]]
			if e.hash == h {
				other := e.ref
				if err := m.validNode(other); err != nil {
					return astNodeRef{}, false, 0, err
				}
				equal, err := m.compareNodes(other, key)
				if err != nil {
					return astNodeRef{}, false, 0, err
				}
				if equal {
					return other, true, i, nil
				}
			}
		}
		perturb >>= 5
		i = (i*5 + perturb + 1) & t.mask
	}
}

func (m *ASTManager) tableFind(key astNodeRef) (astNodeRef, bool, error) {
	if m.table.size == 0 {
		return astNodeRef{}, false, nil
	}
	r, found, _, e := m.tableLookup(key, astTableHash(m.nodeAt(key.slot).hash))
	return r, found, e
}

func (m *ASTManager) tableInsert(key astNodeRef) (astNodeRef, bool, error) {
	t := &m.table
	h := astTableHash(m.nodeAt(key.slot).hash)
	var slot uint64
	haveSlot := false
	if t.indices != nil {
		old, found, s, e := m.tableLookup(key, h)
		if e != nil || found {
			return old, false, e
		}
		slot, haveSlot = s, true
	}
	t.add(h, key, slot, haveSlot)
	return key, true, nil
}

func (m *ASTManager) tableEraseIdentity(key astNodeRef) error {
	return m.tableEraseCaptured(key, m.nodeAt(key.slot).hash)
}

// Matching captured erasure never dereferences a source-freed placement or
// a replacement generation. Only the original exact token is consumed: the
// probe compares entry tokens, as the fork's ast_table::erase compares
// pointers.
func (m *ASTManager) tableEraseCaptured(key astNodeRef, hash uint32) error {
	t := &m.table
	if t.size == 0 {
		return ErrASTHandle
	}
	h := astTableHash(hash)
	tag := astTableTag(h)
	i, perturb := h&t.mask, h
	for t.tags[i] != astTagEmpty {
		if t.tags[i] == tag {
			ix := t.indices[i]
			if e := &t.entries[ix]; e.ref == key && e.hash == h {
				t.indices[i] = astSlotDummy
				t.tags[i] = astTagDummy
				t.entries[ix] = astTableEntry{}
				t.size--
				return nil
			}
		}
		perturb >>= 5
		i = (i*5 + perturb + 1) & t.mask
	}
	return ErrASTHandle
}

// CompactMemory is ast_manager::compact_memory (src/ast/ast.cpp:1483-1497).
// m_alloc.consolidate() has no counterpart: Go owns that storage. A table
// whose capacity exceeds four times its size is rebuilt by re-inserting
// its nodes, in order, into a fresh table.
func (m *ASTManager) CompactMemory() error {
	if err := m.lock(); err != nil {
		return err
	}
	defer m.mu.Unlock()
	t := &m.table
	capacity := t.capacity()
	if uint64(capacity) > 4*uint64(t.size) {
		var fresh astCanonicalTable
		for _, e := range t.entries {
			if e.ref.slot != 0 {
				fresh.add(e.hash, e.ref, 0, false)
			}
		}
		m.table = fresh
		t = &m.table
		if udiag.VerboseProgress {
			return astTableVerbose("(ast-table :prev-capacity " + strconv.FormatUint(uint64(capacity), 10) +
				" :capacity " + strconv.FormatUint(uint64(t.capacity()), 10) + " :size " + strconv.FormatUint(uint64(t.size), 10) + ")\n")
		}
		return nil
	}
	if udiag.VerboseProgress {
		return astTableVerbose("(ast-table :capacity " + strconv.FormatUint(uint64(capacity), 10) +
			" :size " + strconv.FormatUint(uint64(t.size), 10) + ")\n")
	}
	return nil
}

// astTableVerbose writes an IF_VERBOSE(10, ...) line of compact_memory.
func astTableVerbose(text string) error {
	enabled, err := udiag.ProcessVerboseEnabled(10)
	if err != nil || !enabled {
		return err
	}
	out, err := udiag.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	if err = out.AppendCString(text); err != nil {
		return err
	}
	return out.Flush()
}
