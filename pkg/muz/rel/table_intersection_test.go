package rel

import (
	"reflect"
	"testing"
)

func intersectionTestTable(t *testing.T, domains []uint64, facts ...Fact) *HashTable {
	t.Helper()
	signature, err := NewSignature(domains)
	if err != nil {
		t.Fatal(err)
	}
	table := NewHashTable(signature)
	for _, fact := range facts {
		if _, err = table.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	return table
}

func intersectionWantFacts(t *testing.T, table *HashTable, expected ...Fact) {
	t.Helper()
	actual, err := table.Facts()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("facts %v, want %v", actual, expected)
	}
}

func TestHashTableIntersectionMatchesSelectedColumns(t *testing.T) {
	target := intersectionTestTable(t, []uint64{4, 4}, Fact{0, 1}, Fact{1, 2}, Fact{2, 1}, Fact{3, 3})
	source := intersectionTestTable(t, []uint64{4}, Fact{1}, Fact{3})
	if err := target.FilterByIntersection(source, []int{1}, []int{0}); err != nil {
		t.Fatal(err)
	}
	intersectionWantFacts(t, target, Fact{0, 1}, Fact{2, 1}, Fact{3, 3})
}
