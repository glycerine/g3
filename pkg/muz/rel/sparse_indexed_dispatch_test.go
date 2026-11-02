package rel

import "testing"

func TestSparseJoinDispatchesIndexedFunctionalLayoutDeterministically(t *testing.T) {
	left := newSparseFunctionalFixture(t, []uint64{8, 16, 32}, 1,
		Fact{1, 10, 20}, Fact{1, 11, 21}, Fact{2, 12, 22}, Fact{3, 13, 23})
	right := newSparseFunctionalFixture(t, []uint64{8, 64}, 1,
		Fact{1, 40}, Fact{3, 41})
	joined, err := left.Join(right, []int{0}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	defer joined.Close()
	if functional, err := joined.FunctionalColumns(); err != nil || functional != 2 {
		t.Fatalf("functional columns=%d err=%v", functional, err)
	}
	facts, err := joined.Facts()
	if err != nil {
		t.Fatal(err)
	}
	want := []Fact{{1, 10, 1, 20, 40}, {1, 11, 1, 21, 40}, {3, 13, 3, 23, 41}}
	assertSparseKeyFacts(t, facts, want)
}

func TestSparseJoinDispatchPreservesZeroColumnExtension(t *testing.T) {
	signature, err := NewSignature(nil)
	if err != nil {
		t.Fatal(err)
	}
	left, err := NewSparseTable(signature, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer left.Close()
	right, err := NewSparseTable(signature, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer right.Close()
	if _, err = left.Add(Fact{}); err != nil {
		t.Fatal(err)
	}
	if _, err = right.Add(Fact{}); err != nil {
		t.Fatal(err)
	}
	joined, err := left.Join(right, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer joined.Close()
	if rows, err := joined.Facts(); err != nil || len(rows) != 1 || len(rows[0]) != 0 {
		t.Fatalf("zero-column join rows=%v err=%v", rows, err)
	}
}

func TestSparseNegationDispatchPreservesSourceRemovalOrder(t *testing.T) {
	signature, err := NewSignature([]uint64{8, 16})
	if err != nil {
		t.Fatal(err)
	}
	target, err := NewSparseTable(signature, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	for _, fact := range []Fact{{0, 10}, {1, 11}, {2, 12}, {3, 13}} {
		if _, err = target.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	negative, err := NewSparseTable(signature, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer negative.Close()
	for _, fact := range []Fact{{1, 0}, {3, 0}} {
		if _, err = negative.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	if err = target.FilterNegation(negative, []int{0}, []int{0}); err != nil {
		t.Fatal(err)
	}
	facts, err := target.Facts()
	if err != nil {
		t.Fatal(err)
	}
	assertSparseKeyFacts(t, facts, []Fact{{0, 10}, {2, 12}})
}
