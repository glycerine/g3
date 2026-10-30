package rel

import "testing"

func TestSparseIndexedSelectRebuildsAfterMutation(t *testing.T) {
	table := newSparseFunctionalFixture(t, []uint64{4, 4}, 0, Fact{1, 0})
	plan, err := NewSparseIndexedSelectPlan(table, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	first, firstStats, err := plan.Execute(table)
	if err != nil {
		t.Fatal(err)
	}
	_ = first.Close()
	if _, err := table.Add(Fact{1, 2}); err != nil {
		t.Fatal(err)
	}
	second, secondStats, err := plan.Execute(table)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if firstStats.Matches != 1 || secondStats.Matches != 2 {
		t.Fatalf("first=%+v second=%+v", firstStats, secondStats)
	}
	facts, _ := second.Facts()
	assertSparseKeyFacts(t, facts, []Fact{{0}, {2}})
}
