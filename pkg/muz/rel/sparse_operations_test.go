package rel

import "testing"

func TestSparseRenameRejectsClosedTable(t *testing.T) {
	signature, err := NewSignature([]uint64{2, 2})
	must(t, err)
	table, err := NewSparseTable(signature, 0)
	must(t, err)
	must(t, table.Close())
	if _, err = table.Rename([]int{0, 1}); err != ErrTable {
		t.Fatalf("rename on closed table error = %v, want %v", err, ErrTable)
	}
}

func TestSparseJoinProjectRenameFilterUnion(t *testing.T) {
	s, _ := NewSignature([]uint64{4, 4})
	a, _ := NewSparseTable(s, 0)
	b, _ := NewSparseTable(s, 0)
	a.Add(Fact{1, 2})
	a.Add(Fact{3, 1})
	b.Add(Fact{2, 0})
	b.Add(Fact{1, 3})
	j, e := a.JoinProject(b, []int{1}, []int{0}, []int{1, 2})
	must(t, e)
	if n, _ := j.Len(); n != 2 {
		t.Fatal(n)
	}
	r, e := a.Rename([]int{0, 1})
	must(t, e)
	ok, _ := r.Contains(Fact{2, 1})
	if !ok {
		t.Fatal()
	}
	must(t, a.FilterNegation(b, []int{1}, []int{0}))
	if n, _ := a.Len(); n != 0 {
		t.Fatal(n)
	}
	delta, _ := NewSparseTable(s, 0)
	must(t, a.Union(b, delta))
	if n, _ := delta.Len(); n != 2 {
		t.Fatal(n)
	}
}

func TestSparseJoinUsesSourceSmallerOuterTraversal(t *testing.T) {
	signature, err := NewSignature([]uint64{3, 32})
	must(t, err)
	left, err := NewSparseTable(signature, 0)
	must(t, err)
	right, err := NewSparseTable(signature, 0)
	must(t, err)
	for _, fact := range []Fact{{0, 10}, {1, 11}, {2, 12}} {
		if _, err = left.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	for _, fact := range []Fact{{2, 22}, {0, 20}} {
		if _, err = right.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	joined, err := left.Join(right, []int{0}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	facts, err := joined.Facts()
	if err != nil {
		t.Fatal(err)
	}
	want := []Fact{{2, 12, 2, 22}, {0, 10, 0, 20}}
	if len(facts) != len(want) {
		t.Fatalf("join rows = %v, want %v", facts, want)
	}
	for i := range want {
		if len(facts[i]) != len(want[i]) {
			t.Fatalf("join row %d = %v, want %v", i, facts[i], want[i])
		}
		for column := range want[i] {
			if facts[i][column] != want[i][column] {
				t.Fatalf("join row %d = %v, want %v", i, facts[i], want[i])
			}
		}
	}
}

func TestSparseFunctionalJoinUsesSourceSmallerOuterTraversal(t *testing.T) {
	leftSignature, err := NewSignature([]uint64{3, 32, 128})
	must(t, err)
	rightSignature, err := NewSignature([]uint64{3, 32})
	must(t, err)
	left, err := NewSparseTable(leftSignature, 1)
	must(t, err)
	right, err := NewSparseTable(rightSignature, 0)
	must(t, err)
	for _, fact := range []Fact{{0, 10, 100}, {1, 11, 101}, {2, 12, 102}} {
		if _, err = left.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	for _, fact := range []Fact{{2, 22}, {0, 20}} {
		if _, err = right.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	joined, err := left.JoinFunctional(right, []int{0}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	facts, err := joined.Facts()
	if err != nil {
		t.Fatal(err)
	}
	want := []Fact{{2, 12, 2, 22, 102}, {0, 10, 0, 20, 100}}
	if len(facts) != len(want) {
		t.Fatalf("functional join rows = %v, want %v", facts, want)
	}
	for i := range want {
		if len(facts[i]) != len(want[i]) {
			t.Fatalf("functional join row %d = %v, want %v", i, facts[i], want[i])
		}
		for column := range want[i] {
			if facts[i][column] != want[i][column] {
				t.Fatalf("functional join row %d = %v, want %v", i, facts[i], want[i])
			}
		}
	}
}
