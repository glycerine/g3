package ast

import "testing"

// alloc(quantifier_set, 1) (ufbv_rewriter.cpp:226) starts a core_hashtable
// at capacity 1; insert expands before the slot search whenever
// (size + deleted) * 4 > capacity * 3, so capacities run 1, 2, 4, 4, 8.
func TestObjMapCapacityOneGrowthAndErase(t *testing.T) {
	m := astTestManager(t)
	h := markTestNodes(t, m, 5)
	for _, n := range h {
		astTestMust(t, m.IncRef(n))
	}
	defer func() {
		for _, n := range h {
			astTestMust(t, m.DecRef(n))
		}
	}()
	o := NewObjMapCapacity(m, 1)
	if len(o.table) != 1 {
		t.Fatal("initial capacity", len(o.table))
	}
	want := []int{1, 2, 4, 4, 8}
	for i, n := range h {
		astTestMust(t, o.Insert(n, [2]ASTHandle{}))
		if len(o.table) != want[i] || o.Len() != i+1 {
			t.Fatal("capacity after insert", i, len(o.table), o.Len())
		}
	}
	// Iteration is slot order; every key sits at or after its home slot
	// (modulo wrap) with no free slot in between.
	mask := uint32(len(o.table) - 1)
	for i, entry := range o.table {
		if entry.state != objMapUsed {
			continue
		}
		for j := int(entry.hash & mask); j != i; j = (j + 1) & int(mask) {
			if o.table[j].state == objMapFree {
				t.Fatal("probe chain broken", i, j)
			}
		}
	}
	hash, e := m.GetHash(h[2])
	astTestMust(t, e)
	astTestMust(t, o.EraseHash(h[2], hash))
	if found, e := o.Contains(h[2]); e != nil || found || o.Len() != 4 {
		t.Fatal("EraseHash", found, e, o.Len())
	}
	if NewObjMapCapacity(m, 3).table == nil || len(NewObjMapCapacity(m, 3).table) != objMapInitialCapacity {
		t.Fatal("non power-of-two capacity must fall back to the default")
	}
}
