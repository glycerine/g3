package rel

import "testing"

func TestSparseKeyIndexOwnsPlansResultsAndClones(t *testing.T) {
	table := newSparseFunctionalFixture(t, []uint64{4, 4}, 0,
		Fact{1, 2}, Fact{1, 3})
	columns := []int{0}
	plan, err := NewSparseKeyLookupPlan(table, columns)
	if err != nil {
		t.Fatal(err)
	}
	columns[0] = 1
	returned := plan.Columns()
	returned[0] = 1
	if plan.Columns()[0] != 0 {
		t.Fatalf("plan columns aliased: %v", plan.Columns())
	}
	snapshot, _, err := plan.Build(table)
	if err != nil {
		t.Fatal(err)
	}
	rows, _ := snapshot.Lookup(Fact{1})
	rows[0][0] = 3
	groups := snapshot.Groups()
	groups[0].Key[0] = 3
	groups[0].Rows[0][0] = 3
	clone := snapshot.Clone()
	clone.groups[0].Rows[0][0] = 2
	unchanged, _ := snapshot.Lookup(Fact{1})
	assertSparseKeyFacts(t, unchanged, []Fact{{1, 2}, {1, 3}})
}
