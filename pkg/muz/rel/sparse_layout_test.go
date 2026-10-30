package rel

import "testing"

func TestSparseLayoutWideAndFunctionalAlignment(t *testing.T) {
	signature, err := NewSignature([]uint64{3, 1 << 55, 17, 2})
	if err != nil {
		t.Fatal(err)
	}
	fact := Fact{2, Element((uint64(1) << 55) - 1), 16, 1}
	tests := []struct {
		name              string
		functionalColumns int
		entryBytes        uint32
		functionalBytes   uint32
	}{
		{"no functional suffix", 0, 9, 0},
		{"two-column functional suffix", 2, 9, 1},
		{"single functional column", 1, 10, 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			table, err := NewSparseTable(signature, test.functionalColumns)
			if err != nil {
				t.Fatal(err)
			}
			defer table.Close()
			if table.layout.EntryBytes() != test.entryBytes || table.layout.functionalBytes != test.functionalBytes {
				t.Fatalf("layout bytes = (%d, %d), want (%d, %d)", table.layout.EntryBytes(), table.layout.functionalBytes, test.entryBytes, test.functionalBytes)
			}
			added, err := table.Add(fact)
			if err != nil || !added {
				t.Fatalf("insert wide fact: added=%v err=%v", added, err)
			}
			facts, err := table.Facts()
			if err != nil || len(facts) != 1 {
				t.Fatalf("packed round trip: facts=%v err=%v", facts, err)
			}
			for i := range fact {
				if facts[0][i] != fact[i] {
					t.Fatalf("packed round trip: facts=%v, want %v", facts, fact)
				}
			}
		})
	}
}

func TestSparseLayoutFullWidthColumnMask(t *testing.T) {
	signature, err := NewSignature([]uint64{^uint64(0)})
	if err != nil {
		t.Fatal(err)
	}
	table, err := NewSparseTable(signature, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer table.Close()
	if table.layout.EntryBytes() != 8 {
		t.Fatalf("64-bit column entry size: got %d, want 8", table.layout.EntryBytes())
	}
	fact := Fact{Element(^uint64(0) - 1)}
	if added, err := table.Add(fact); err != nil || !added {
		t.Fatalf("insert maximum representable value: added=%v err=%v", added, err)
	}
	got, err := table.Facts()
	if err != nil || len(got) != 1 || got[0][0] != fact[0] {
		t.Fatalf("64-bit round trip: facts=%v err=%v", got, err)
	}
}
