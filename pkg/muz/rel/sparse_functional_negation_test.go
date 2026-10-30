package rel

import (
	"errors"
	"testing"
)

func TestSparseFunctionalNegationUsesOnlyNonFunctionalIndexes(t *testing.T) {
	target := newSparseFunctionalFixture(t, []uint64{4, 8}, 1, Fact{1, 2}, Fact{2, 3})
	negative := newSparseFunctionalFixture(t, []uint64{4, 8}, 1, Fact{1, 7})
	removed, err := target.FilterNegationFunctional(negative, []int{0}, []int{0})
	if err != nil || removed != 1 {
		t.Fatalf("removed=%d err=%v", removed, err)
	}
	if exact, _ := target.ContainsExact(Fact{2, 3}); !exact {
		t.Fatal("unmatched row removed")
	}
	if _, err := target.FilterNegationFunctional(negative, []int{1}, []int{1}); !errors.Is(err, ErrTable) {
		t.Fatalf("functional index=%v", err)
	}
}

func TestSparseNegationRemovesMatchingRowsInSourceOffsetOrder(t *testing.T) {
	targetSignature, err := NewSignature([]uint64{4, 2, 4})
	must(t, err)
	negativeSignature, err := NewSignature([]uint64{2})
	must(t, err)
	target, err := NewSparseTable(targetSignature, 1)
	must(t, err)
	negative, err := NewSparseTable(negativeSignature, 0)
	must(t, err)
	for _, fact := range []Fact{{0, 0, 0}, {1, 0, 1}, {2, 1, 2}, {3, 1, 3}} {
		if _, err = target.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = negative.Add(Fact{0}); err != nil {
		t.Fatal(err)
	}
	removed, err := target.FilterNegationFunctional(negative, []int{1}, []int{0})
	if err != nil || removed != 2 {
		t.Fatalf("removed=%d err=%v", removed, err)
	}
	facts, err := target.Facts()
	if err != nil {
		t.Fatal(err)
	}
	want := []Fact{{2, 1, 2}, {3, 1, 3}}
	if len(facts) != len(want) {
		t.Fatalf("negation rows = %v, want %v", facts, want)
	}
	for i := range want {
		for column := range want[i] {
			if facts[i][column] != want[i][column] {
				t.Fatalf("negation row %d = %v, want %v", i, facts[i], want[i])
			}
		}
	}
}

func TestSparseNegationFallsBackForFunctionalJoinColumns(t *testing.T) {
	signature, err := NewSignature([]uint64{2, 2})
	must(t, err)
	target, err := NewSparseTable(signature, 1)
	must(t, err)
	negative, err := NewSparseTable(signature, 1)
	must(t, err)
	for _, fact := range []Fact{{0, 0}, {1, 1}} {
		if _, err = target.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = negative.Add(Fact{0, 0}); err != nil {
		t.Fatal(err)
	}
	if err = target.FilterNegation(negative, []int{1}, []int{1}); err != nil {
		t.Fatal(err)
	}
	facts, err := target.Facts()
	if err != nil || len(facts) != 1 || facts[0][0] != 1 || facts[0][1] != 1 {
		t.Fatalf("functional-column fallback rows = %v, %v", facts, err)
	}
}
