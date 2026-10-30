package rel

import "testing"

func TestSparseKeyIndexFunctionalColumnAndRebuild(t *testing.T) {
	table := newSparseFunctionalFixture(t, []uint64{4, 8}, 1,
		Fact{1, 2}, Fact{2, 3})
	plan, err := NewSparseKeyLookupPlan(table, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	before, _, err := plan.Build(table)
	if err != nil {
		t.Fatal(err)
	}
	if err := table.EnsureFact(Fact{1, 5}); err != nil {
		t.Fatal(err)
	}
	oldRows, err := before.Lookup(Fact{1})
	if err != nil {
		t.Fatal(err)
	}
	assertSparseKeyFacts(t, oldRows, []Fact{{1, 2}})
	after, _, err := plan.Build(table)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := after.Lookup(Fact{1})
	if err != nil {
		t.Fatal(err)
	}
	assertSparseKeyFacts(t, rows, []Fact{{1, 5}})
}
