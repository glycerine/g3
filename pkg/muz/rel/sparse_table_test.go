package rel

import "testing"

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestSparseTablePackedFunctionalStorage(t *testing.T) {
	s, _ := NewSignature([]uint64{3, 5, 2})
	x, e := NewSparseTable(s, 1)
	must(t, e)
	if x.layout.EntryBytes() != 2 {
		t.Fatal(x.layout)
	}
	added, e := x.Add(Fact{2, 4, 1})
	must(t, e)
	if !added {
		t.Fatal()
	}
	added, e = x.Add(Fact{2, 4, 0})
	must(t, e)
	if added {
		t.Fatal("functional key duplicated")
	}
	ok, e := x.Contains(Fact{2, 4, 1})
	must(t, e)
	if !ok {
		t.Fatal()
	}
	if ok, e = x.Contains(Fact{2, 4, 0}); e != nil || ok {
		t.Fatalf("same functional key with a different suffix: found=%v err=%v", ok, e)
	}
	facts, e := x.Facts()
	must(t, e)
	if len(facts) != 1 || facts[0][2] != 1 {
		t.Fatal(facts)
	}
}

func TestSparseTableExactRowEstimate(t *testing.T) {
	signature, err := NewSignature([]uint64{3})
	must(t, err)
	table, err := NewSparseTable(signature, 0)
	must(t, err)
	if rows, err := table.SizeEstimateRows(); err != nil || rows != 0 {
		t.Fatalf("empty row estimate = %d, %v", rows, err)
	}
	if known, err := table.KnowsExactSize(); err != nil || !known {
		t.Fatalf("empty exact-size knowledge = %v, %v", known, err)
	}
	_, err = table.Add(Fact{2})
	must(t, err)
	if rows, err := table.SizeEstimateRows(); err != nil || rows != 1 {
		t.Fatalf("populated row estimate = %d, %v", rows, err)
	}
}

func TestSparseTableSizeEstimateBytes(t *testing.T) {
	signature, err := NewSignature([]uint64{3, 5})
	must(t, err)
	table, err := NewSparseTable(signature, 0)
	must(t, err)
	if bytes, err := table.SizeEstimateBytes(); err != nil || bytes != 0 {
		t.Fatalf("empty byte estimate = %d, %v", bytes, err)
	}
	if _, err := table.Add(Fact{2, 4}); err != nil {
		t.Fatal(err)
	}
	if bytes, err := table.SizeEstimateBytes(); err != nil || bytes != table.layout.EntryBytes() {
		t.Fatalf("populated byte estimate = %d, %v", bytes, err)
	}
}
