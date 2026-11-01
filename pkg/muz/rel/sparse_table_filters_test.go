package rel

import (
	"reflect"
	"testing"
)

func TestSparseTableDefaultEqualFilterHandlesFunctionalSuffix(t *testing.T) {
	signature, err := NewSignature([]uint64{4, 3, 3})
	must(t, err)
	table, err := NewSparseTable(signature, 1)
	must(t, err)
	defer table.Close()
	for _, fact := range []Fact{{0, 0, 1}, {1, 0, 2}, {2, 1, 1}, {3, 2, 2}} {
		if _, err = table.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	if err = table.FilterEqual(1, 2); err != nil {
		t.Fatal(err)
	}
	facts, err := table.Facts()
	if err != nil || !reflect.DeepEqual(facts, []Fact{{0, 0, 1}, {2, 1, 1}}) {
		t.Fatalf("equal-filter facts = %v, err=%v", facts, err)
	}
	// The default source filter compares the raw element and removes every
	// row when the requested element is outside the column domain.
	if err = table.FilterEqual(3, 2); err != nil {
		t.Fatal(err)
	}
	if rows, err := table.Len(); err != nil || rows != 0 {
		t.Fatalf("out-of-domain equal-filter rows = %d, err=%v", rows, err)
	}
}

func TestSparseTableDefaultIdenticalFilterAndValidation(t *testing.T) {
	signature, err := NewSignature([]uint64{3, 3, 3})
	must(t, err)
	table, err := NewSparseTable(signature, 0)
	must(t, err)
	defer table.Close()
	for _, fact := range []Fact{{0, 0, 2}, {1, 0, 2}, {2, 1, 2}, {0, 0, 0}} {
		if _, err = table.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	if err = table.FilterIdentical([]int{1, 0}); err != nil {
		t.Fatal(err)
	}
	facts, err := table.Facts()
	if err != nil || !reflect.DeepEqual(facts, []Fact{{0, 0, 2}, {0, 0, 0}}) {
		t.Fatalf("identical-filter facts = %v, err=%v", facts, err)
	}
	before := append([]Fact(nil), facts...)
	if err = table.FilterIdentical([]int{0, 3}); err == nil {
		t.Fatal("out-of-range column accepted")
	}
	facts, err = table.Facts()
	if err != nil || !reflect.DeepEqual(facts, before) {
		t.Fatalf("invalid filter changed rows: facts=%v err=%v", facts, err)
	}
	if err = table.FilterIdentical([]int{1}); err != nil {
		t.Fatalf("one-column identity filter: %v", err)
	}
}
