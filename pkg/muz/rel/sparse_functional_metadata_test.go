package rel

import "testing"

func newSparseFunctionalFixture(t *testing.T, domains []uint64, functional int, facts ...Fact) *SparseTable {
	t.Helper()
	signature, err := NewSignature(domains)
	if err != nil {
		t.Fatal(err)
	}
	table, err := NewSparseTable(signature, functional)
	if err != nil {
		t.Fatal(err)
	}
	for _, fact := range facts {
		if _, err = table.Add(fact); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { _ = table.Close() })
	return table
}

func TestSparseFunctionalMetadataMatchesSourceBoundary(t *testing.T) {
	table := newSparseFunctionalFixture(t, []uint64{4, 8, 16}, 2)
	functional, err := table.FunctionalColumns()
	first, firstErr := table.FirstFunctionalColumn()
	if err != nil || firstErr != nil || functional != 2 || first != 1 {
		t.Fatalf("functional=%d first=%d err=%v/%v", functional, first, err, firstErr)
	}
}
