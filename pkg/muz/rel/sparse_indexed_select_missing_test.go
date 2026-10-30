package rel

import "testing"

func TestSparseIndexedSelectReturnsOwnedEmptyResult(t *testing.T) {
	table := newSparseFunctionalFixture(t, []uint64{4, 4}, 0, Fact{1, 2})
	plan, err := NewSparseIndexedSelectPlan(table, 3, 0)
	if err != nil {
		t.Fatal(err)
	}
	result, stats, err := plan.Execute(table)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	length, err := result.Len()
	if err != nil || length != 0 || stats.Matches != 0 || stats.Inserted != 0 {
		t.Fatalf("len=%d stats=%+v err=%v", length, stats, err)
	}
}
