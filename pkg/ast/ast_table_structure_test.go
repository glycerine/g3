package ast

import (
	"errors"
	"testing"
)

func TestASTNativeCanonicalTableGrowthCollisionIdentityAndGeneration(t *testing.T) {
	m := astTestManager(t)
	old := m.table
	m.initTable()
	const count = 257
	keys := make([]astNodeRef, count+1)
	defer func() {
		m.BudgetField.FailAfter(-1)
		m.table = old
		for _, key := range keys {
			if key.slot != 0 {
				astTestMust(t, m.freeNodeStorage(key))
			}
		}
	}()
	for i := range keys {
		r, e := m.allocateNode(ASTVar)
		astTestMust(t, e)
		keys[i] = r
		index := uint32(i)
		if i == count {
			index = count - 1
		}
		*m.varAt(r.slot) = astVarRecord{index: index, sort: m.boolSort}
		// Colliding project hashes exercise real structural equality. The Go
		// containers choose their own capacity and bucket placement.
		m.nodeAt(r.slot).hash = 0
	}
	before, physical := m.BudgetField.Used(), m.Stats()
	m.BudgetField.FailAfter(0)
	for _, key := range keys[:count] {
		found, fresh, e := m.tableInsert(key)
		astTestMust(t, e)
		if !fresh || found != key {
			t.Fatal("different variable indices must remain distinct", key, found)
		}
	}
	index := 0
	for _, key := range m.table.entries.All() {
		if key != keys[index] {
			t.Fatal("native canonical order changed", index, key)
		}
		index++
	}
	found, fresh, e := m.tableInsert(keys[count])
	astTestMust(t, e)
	if fresh || found != keys[count-1] || m.table.size != count || index != count || m.tableHashCount(0) != count {
		t.Fatal("native duplicate canonicalization", found, fresh, m.table.size, index)
	}
	if e := m.tableEraseIdentity(keys[count]); !errors.Is(e, ErrASTHandle) {
		t.Fatal("erasure must use identity rather than structural equality", e)
	}
	astTestMust(t, m.tableEraseIdentity(keys[1]))
	found, yes, e := m.tableFind(keys[count])
	astTestMust(t, e)
	if !yes || found != keys[count-1] || m.table.size != count-1 || m.BudgetField.Used() != before || m.Stats() != physical {
		t.Fatal("native collision erase changed other nodes or actual storage", found, yes)
	}
	m.BudgetField.FailAfter(-1)
	// Capture an actual freed placement, then reuse its slab slot. Erasure
	// must consume only the old token without inspecting the replacement.
	original := keys[2]
	astTestMust(t, m.freeNodeStorage(original))
	replacement, e := m.allocateNode(ASTVar)
	astTestMust(t, e)
	keys[2] = replacement
	if replacement.slot != original.slot || replacement.generation == original.generation {
		t.Fatal("actual slab generation reuse", original, replacement)
	}
	*m.varAt(replacement.slot) = astVarRecord{index: count + 9, sort: m.boolSort}
	m.nodeAt(replacement.slot).hash = 0
	astTestMust(t, m.tableEraseCaptured(original, 0))
	_, fresh, e = m.tableInsert(replacement)
	astTestMust(t, e)
	if !fresh {
		t.Fatal("replacement not independently canonical")
	}
	if e := m.tableEraseCaptured(original, 0); !errors.Is(e, ErrASTHandle) {
		t.Fatal("old captured erasure consumed a later generation", e)
	}
	found, yes, e = m.tableFind(replacement)
	astTestMust(t, e)
	if !yes || found != replacement {
		t.Fatal("later generation lost", found, yes)
	}
}

// tableHashCount counts the canonical index entries carrying hash.
func (m *ASTManager) tableHashCount(hash uint32) int {
	n := 0
	for _, s := range m.table.slots {
		if s.ref.slot != 0 && s.hash == hash {
			n++
		}
	}
	return n
}
