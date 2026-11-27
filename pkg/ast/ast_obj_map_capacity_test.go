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

func TestObjMapEmptyCopyAndPairErase(t *testing.T) {
	m := astTestManager(t)
	h := markTestNodes(t, m, 2)
	o := NewObjMap(m)
	clone := o.Copy()
	for _, table := range []*ObjMap{o, clone} {
		table.Reset()
		table.Finalize()
		table.ErasePairKeys(h[0], h[1], 13)
		if table.Len() != 0 || table.Capacity() != objMapInitialCapacity || len(table.Entries()) != 0 {
			t.Fatal("empty table lost its initial capacity or acquired entries")
		}
	}
	clone.InsertPairKeys(h[0], h[1], 13)
	if o.Len() != 0 || clone.Len() != 1 {
		t.Fatal("inserting into empty copy changed original")
	}
	clone.ErasePairKeys(h[0], h[1], 13)
	if clone.Len() != 0 {
		t.Fatal("pair survived erase")
	}
}

// Keep the default-capacity layout as the control for initially empty tables.
func BenchmarkObjMapEmptyWorkers(b *testing.B) {
	m := astTestManager(b)
	for _, mode := range []string{"Explicit", "Default"} {
		b.Run(mode, func(b *testing.B) {
			var maps [1024]*ObjMap
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				for j := range maps {
					if mode == "Default" {
						maps[j] = NewObjMap(m)
					} else {
						maps[j] = NewObjMapCapacity(m, objMapInitialCapacity)
					}
				}
				if maps[1023].Capacity() != objMapInitialCapacity {
					b.Fatal("wrong capacity")
				}
			}
		})
	}
}
