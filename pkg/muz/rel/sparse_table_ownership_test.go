package rel

import "testing"

func TestSparseTableOwnsPackedFactsAndClonesIndependently(t *testing.T) {
	signature, err := NewSignature([]uint64{4, 4})
	must(t, err)
	table, err := NewSparseTable(signature, 0)
	must(t, err)
	defer table.Close()

	input := Fact{1, 2}
	if added, err := table.Add(input); err != nil || !added {
		t.Fatalf("add input fact: added=%v err=%v", added, err)
	}
	input[0], input[1] = 3, 3
	rows, err := table.Facts()
	if err != nil || len(rows) != 1 || rows[0][0] != 1 || rows[0][1] != 2 {
		t.Fatalf("input fact aliased stored data: rows=%v err=%v", rows, err)
	}
	rows[0][0] = 0
	rows, err = table.Facts()
	if err != nil || len(rows) != 1 || rows[0][0] != 1 || rows[0][1] != 2 {
		t.Fatalf("returned fact aliased stored data: rows=%v err=%v", rows, err)
	}

	clone, err := table.Clone()
	must(t, err)
	defer clone.Close()
	if removed, err := table.Remove(Fact{1, 2}); err != nil || !removed {
		t.Fatalf("remove source row: removed=%v err=%v", removed, err)
	}
	if added, err := table.Add(Fact{3, 1}); err != nil || !added {
		t.Fatalf("mutate source after clone: added=%v err=%v", added, err)
	}
	cloneRows, err := clone.Facts()
	if err != nil || len(cloneRows) != 1 || cloneRows[0][0] != 1 || cloneRows[0][1] != 2 {
		t.Fatalf("clone shared mutable row storage: rows=%v err=%v", cloneRows, err)
	}
	if added, err := clone.Add(Fact{2, 0}); err != nil || !added {
		t.Fatalf("mutate clone independently: added=%v err=%v", added, err)
	}
	if rows, err = table.Facts(); err != nil || len(rows) != 1 || rows[0][0] != 3 || rows[0][1] != 1 {
		t.Fatalf("clone mutation leaked to source: rows=%v err=%v", rows, err)
	}
}
