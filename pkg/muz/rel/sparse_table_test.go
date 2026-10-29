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
	facts, e := x.Facts()
	must(t, e)
	if len(facts) != 1 || facts[0][2] != 1 {
		t.Fatal(facts)
	}
}
