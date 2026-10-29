package sat

import "testing"

func testSATCut(ids []uint32, table uint64) satCut {
	var c satCut
	for _, id := range ids {
		if !c.Add(id) {
			panic("test cut too large")
		}
	}
	c.SetTable(table)
	return c
}

func TestSATCutMergeSubsetRemoveAndTables(t *testing.T) {
	a := testSATCut([]uint32{1, 4, 7}, 0xa5)
	b := testSATCut([]uint32{2, 4, 9, 11}, 0x5a5a)
	var merged satCut
	if merged.Merge(a, b) {
		t.Fatal("six-element union should exceed retained five-input bound")
	}
	b = testSATCut([]uint32{4, 7}, 6)
	merged = satCut{}
	if !merged.Merge(a, b) || !merged.DomainEqual(testSATCut([]uint32{1, 4, 7}, 0)) {
		t.Fatal("sorted union", merged)
	}
	if !b.SubsetOf(a) || a.SubsetOf(b) || testSATCut([]uint32{1, 8}, 0).SubsetOf(a) {
		t.Fatal("subset relation")
	}
	copyA := a
	if !a.Equal(copyA) || a.Hash() != copyA.Hash() || a.DomainHash() != copyA.DomainHash() || (satCut{}).Hash() != 11 {
		t.Fatal("cut hash/equality contract")
	}

	c := testSATCut([]uint32{3, 8, 13}, 0b11001010)
	if !c.RemoveElem(1) || !c.DomainEqual(testSATCut([]uint32{3, 13}, 0)) || c.Table() != 0b0010 {
		t.Fatalf("remove middle input: %v table %04b", c, c.Table())
	}
	if c.RemoveElem(2) {
		t.Fatal("out-of-range removal")
	}
	c.AddDontCare(0xd)
	if c.Table() != 15 || c.NTable() != 0xd || !c.IsTrue() || c.IsFalse() {
		t.Fatalf("don't-care table semantics: table=%x ntable=%x", c.Table(), c.NTable())
	}
	c.Negate()
	if c.Table() != 0xd || c.String() != "{3 13} 1011" {
		t.Fatal("negate/display", c.String())
	}
}

func TestSATCutEffectMasks(t *testing.T) {
	want := []uint64{0x5555555555555555, 0x3333333333333333, 0x0f0f0f0f0f0f0f0f, 0x00ff00ff00ff00ff}
	for i, w := range want {
		if got := satCutEffectMask(uint32(i)); got != w {
			t.Fatalf("mask %d = %016x, want %016x", i, got, w)
		}
	}
}
