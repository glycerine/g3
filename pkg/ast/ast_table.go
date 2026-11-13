package ast

// Source: src/ast/ast.h:920-924; src/ast/ast.cpp:1714-1780.

import "github.com/glycerine/insdict"

// Canonical entries borrow exact node tokens. An open-addressed index on the
// node hash accelerates structural comparison; dictionary order makes
// teardown deterministic. The table holds at most one node per structure,
// so the probe order among equal hashes never decides a lookup.
type astCanonicalTable struct {
	entries insdict.Dict[uint64, astNodeRef]
	// slots is a linear-probing index, its length a power of two (or nil
	// when empty). A slot with ref.slot == 0 is free.
	slots []astTableSlot
	shift uint32 // 32 - log2(len(slots))
	size  uint32
}

type astTableSlot struct {
	hash uint32
	ref  astNodeRef
}

const astTableMinSlots = 64

func astTableKey(r astNodeRef) uint64 {
	return uint64(r.slot)<<32 | uint64(r.generation)
}

// home spreads the node hash over the index (multiplicative hashing).
func (t *astCanonicalTable) home(hash uint32) uint32 {
	return (hash * 0x9e3779b1) >> t.shift
}

func (m *ASTManager) initTable() {
	m.table = astCanonicalTable{}
}

func (m *ASTManager) tableFind(key astNodeRef) (astNodeRef, bool, error) {
	t := &m.table
	if len(t.slots) == 0 {
		return astNodeRef{}, false, nil
	}
	hash := m.nodeAt(key.slot).hash
	mask := uint32(len(t.slots) - 1)
	for i := t.home(hash); ; i = (i + 1) & mask {
		s := &t.slots[i]
		if s.ref.slot == 0 {
			return astNodeRef{}, false, nil
		}
		if s.hash != hash {
			continue
		}
		other := s.ref
		if e := m.validNode(other); e != nil {
			return astNodeRef{}, false, e
		}
		equal, e := m.compareNodes(other, key)
		if e != nil {
			return astNodeRef{}, false, e
		}
		if equal {
			return other, true, nil
		}
	}
}

func (t *astCanonicalTable) place(hash uint32, ref astNodeRef) {
	mask := uint32(len(t.slots) - 1)
	i := t.home(hash)
	for t.slots[i].ref.slot != 0 {
		i = (i + 1) & mask
	}
	t.slots[i] = astTableSlot{hash: hash, ref: ref}
}

func (t *astCanonicalTable) grow() {
	n := 2 * len(t.slots)
	if n < astTableMinSlots {
		n = astTableMinSlots
	}
	old := t.slots
	t.slots = make([]astTableSlot, n)
	t.shift = 32
	for k := n; k > 1; k >>= 1 {
		t.shift--
	}
	for _, s := range old {
		if s.ref.slot != 0 {
			t.place(s.hash, s.ref)
		}
	}
}

func (m *ASTManager) tableInsert(key astNodeRef) (astNodeRef, bool, error) {
	if old, found, e := m.tableFind(key); e != nil || found {
		return old, false, e
	}
	t := &m.table
	if 4*(uint64(t.size)+1) > 3*uint64(len(t.slots)) {
		t.grow()
	}
	t.place(m.nodeAt(key.slot).hash, key)
	t.entries.Put(astTableKey(key), key)
	t.size++
	return key, true, nil
}

func (m *ASTManager) tableEraseIdentity(key astNodeRef) error {
	return m.tableEraseCaptured(key, m.nodeAt(key.slot).hash)
}

// Matching captured erasure never dereferences a source-freed placement or
// a replacement generation. Only the original exact token is consumed.
func (m *ASTManager) tableEraseCaptured(key astNodeRef, hash uint32) error {
	t := &m.table
	if !t.unindex(key, hash) {
		return ErrASTHandle
	}
	t.entries.DelPackMaybe(astTableKey(key))
	t.size--
	return nil
}

// unindex removes the exact token key from the index, reporting whether it
// was present.
func (t *astCanonicalTable) unindex(key astNodeRef, hash uint32) bool {
	if len(t.slots) == 0 {
		return false
	}
	mask := uint32(len(t.slots) - 1)
	i := t.home(hash)
	for {
		s := t.slots[i]
		if s.ref.slot == 0 {
			return false
		}
		if s.ref == key && s.hash == hash {
			break
		}
		i = (i + 1) & mask
	}
	// Backward-shift deletion keeps every remaining entry reachable.
	for {
		t.slots[i] = astTableSlot{}
		j := i
		for {
			j = (j + 1) & mask
			s := t.slots[j]
			if s.ref.slot == 0 {
				return true
			}
			home := t.home(s.hash)
			// Move s into the hole at i unless its home lies cyclically in (i, j].
			if (j > i && (home <= i || home > j)) || (j < i && (home <= i && home > j)) {
				t.slots[i] = s
				i = j
				break
			}
		}
	}
}
