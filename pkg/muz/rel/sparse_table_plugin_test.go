package rel

import "testing"

// TestSparseTablePluginPoolReusesRecycledStorage checks sparse_table_plugin's
// pool: recycle resets and pools a table, mk_empty reuses the last pooled
// table of the signature with its grown capacities, and mk_clone assigns the
// source's m_data (capacity included).
func TestSparseTablePluginPoolReusesRecycledStorage(t *testing.T) {
	plugin := newSparseTablePlugin()
	signature, err := NewSignature([]uint64{256, 256})
	must(t, err)
	table, err := plugin.MkEmpty(signature, 0)
	must(t, err)
	for i := 0; i < 20; i++ {
		if added, err := table.Add(Fact{Element(i), Element(i + 1)}); err != nil || !added {
			t.Fatalf("add %d: %v %v", i, added, err)
		}
	}
	// 20 two-byte rows: m_data grows to 48 bytes (capacity 62); the indexer
	// expands to 32 slots.
	if got := table.sizeEstimateBytes(); got != 62+32*16+8*8 {
		t.Fatalf("filled estimate = %d", got)
	}
	clone, err := table.Clone()
	must(t, err)
	if got := clone.sizeEstimateBytes(); got != 62+32*16+8*8 {
		t.Fatalf("clone estimate = %d", got)
	}
	facts, err := clone.Facts()
	must(t, err)
	if len(facts) != 20 || facts[3][0] != 3 || facts[3][1] != 4 {
		t.Fatalf("clone rows = %v", facts)
	}
	must(t, table.Close())
	if _, err := table.Len(); err == nil {
		t.Fatal("recycled table handle stays open")
	}
	// reset keeps the data capacity; the 32-slot indexer had 12 free slots, too few to halve.
	pooled, err := plugin.MkEmpty(signature, 0)
	must(t, err)
	if got := pooled.sizeEstimateBytes(); got != 62+32*16+8*8 {
		t.Fatalf("pooled estimate = %d", got)
	}
	if empty, err := pooled.Empty(); err != nil || !empty {
		t.Fatalf("pooled table not empty: %v %v", empty, err)
	}
	fresh, err := plugin.MkEmpty(signature, 0)
	must(t, err)
	if got := fresh.sizeEstimateBytes(); got != 200 {
		t.Fatalf("fresh estimate = %d", got)
	}
	must(t, fresh.Close())
	must(t, pooled.Close())
	must(t, clone.Close())
	plugin.reset()
}

// TestSparseTableRemoveMovesLastRow checks entry_storage::remove_offset: the
// last row takes the removed row's place.
func TestSparseTableRemoveMovesLastRow(t *testing.T) {
	signature, err := NewSignature([]uint64{16})
	must(t, err)
	table, err := NewSparseTable(signature, 0)
	must(t, err)
	defer table.Close()
	for i := 0; i < 5; i++ {
		_, err := table.Add(Fact{Element(i)})
		must(t, err)
	}
	removed, err := table.Remove(Fact{1})
	must(t, err)
	if !removed {
		t.Fatal("row 1 not removed")
	}
	facts, err := table.Facts()
	must(t, err)
	want := []Element{0, 4, 2, 3}
	if len(facts) != len(want) {
		t.Fatalf("rows = %v", facts)
	}
	for i, f := range facts {
		if f[0] != want[i] {
			t.Fatalf("rows = %v, want %v", facts, want)
		}
	}
	if found, err := table.Contains(Fact{4}); err != nil || !found {
		t.Fatalf("moved row lost: %v %v", found, err)
	}
	if found, err := table.Contains(Fact{1}); err != nil || found {
		t.Fatalf("removed row found: %v %v", found, err)
	}
}
