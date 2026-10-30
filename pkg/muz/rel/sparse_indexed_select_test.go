package rel

import "testing"

func TestSparseIndexedSelectProjectsMatchesInSourceOrder(t *testing.T) {
	table := newSparseFunctionalFixture(t, []uint64{4, 4, 4}, 0,
		Fact{1, 2, 0}, Fact{0, 2, 1}, Fact{2, 3, 2})
	plan, err := NewSparseIndexedSelectPlan(table, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	result, stats, err := plan.Execute(table)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	if stats.Rows != 3 || stats.IndexGroups != 2 || stats.Matches != 2 || stats.Inserted != 2 || stats.FullSignature {
		t.Fatalf("stats=%+v", stats)
	}
	facts, _ := result.Facts()
	assertSparseKeyFacts(t, facts, []Fact{{1, 0}, {0, 1}})
}
