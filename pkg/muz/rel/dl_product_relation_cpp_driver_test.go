package rel

// Source: src/test/dl_product_relation.cpp test_functional_columns.

import "testing"

func TestProductRelationSourceFunctionalRows(t *testing.T) {
	table := newSparseFunctionalFixture(t, []uint64{2, 2}, 1)
	for i, fact := range []Fact{{0, 0}, {0, 1}, {1, 1}} {
		if _, err := table.Add(fact); err != nil {
			t.Fatal(err)
		}
		want := []uint32{1, 1, 2}[i]
		if n, err := table.SizeEstimateRows(); err != nil || n != want {
			t.Fatal("row count", i, n, err)
		}
	}
	for _, removed := range []int{0, 1} {
		projected, err := table.Project([]int{removed})
		if err != nil {
			t.Fatal(err)
		}
		if n, err := projected.SizeEstimateRows(); err != nil || n != 2 {
			t.Fatal("projection", removed, n, err)
		}
		if removed == 0 {
			if n, err := projected.FunctionalColumns(); err != nil || n != 0 {
				t.Fatal("projected functional count", n, err)
			}
		}
		if err := projected.Close(); err != nil {
			t.Fatal(err)
		}
	}
	var reductions [][2][]Element
	projectedWithReduce, err := table.ProjectWithReduce([]int{0}, func(existing, incoming []Element) {
		reductions = append(reductions, [2][]Element{append([]Element(nil), existing...), append([]Element(nil), incoming...)})
	})
	if err != nil {
		t.Fatal(err)
	}
	defer projectedWithReduce.Close()
	if n, err := projectedWithReduce.SizeEstimateRows(); err != nil || n != 1 {
		t.Fatal("reduced projection rows", n, err)
	}
	if functional, err := projectedWithReduce.FunctionalColumns(); err != nil || functional != 1 {
		t.Fatal("reduced projection functional columns", functional, err)
	}
	if got, err := projectedWithReduce.Contains(Fact{0}); err != nil || !got {
		t.Fatal("reduced projection kept row", got, err)
	}
	if len(reductions) != 1 || len(reductions[0][0]) != 1 || reductions[0][0][0] != 0 || reductions[0][1][0] != 1 {
		t.Fatalf("reducer collision values=%v", reductions)
	}
}

func TestProductRelationSourceSuggestEnsure(t *testing.T) {
	table := newSparseFunctionalFixture(t, []uint64{2, 2}, 1)
	first := Fact{0, 1}
	if added, err := table.SuggestFact(first); err != nil || !added || first[1] != 1 {
		t.Fatal("first suggestion", first, added, err)
	}
	second := Fact{0, 0}
	if added, err := table.SuggestFact(second); err != nil || added || second[1] != 1 {
		t.Fatal("existing suggestion", second, added, err)
	}
	if err := table.EnsureFact(Fact{0, 0}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		fact Fact
		want bool
	}{{Fact{0, 0}, true}, {Fact{0, 1}, false}} {
		if got, err := table.Contains(tc.fact); err != nil || got != tc.want {
			t.Fatal("ensured fact", tc.fact, got, err)
		}
	}
}

func TestProductRelationSourceFunctionalColumnJoin(t *testing.T) {
	table := newSparseFunctionalFixture(t, []uint64{2, 2}, 1, Fact{0, 1})
	joined, err := table.Join(table, []int{1}, []int{1})
	if err != nil {
		t.Fatal(err)
	}
	defer joined.Close()
	if n, err := joined.FunctionalColumns(); err != nil || n != 2 {
		t.Fatal("functional columns", n, err)
	}
	for _, tc := range []struct {
		fact Fact
		want bool
	}{{Fact{0, 0, 1, 1}, true}, {Fact{0, 1, 1, 1}, false}} {
		if got, err := joined.Contains(tc.fact); err != nil || got != tc.want {
			t.Fatal("join fact", tc.fact, got, err)
		}
	}
	projected, err := table.JoinProject(table, []int{1}, []int{1}, []int{3})
	if err != nil {
		t.Fatal(err)
	}
	defer projected.Close()
	if n, err := projected.FunctionalColumns(); err != nil || n != 1 {
		t.Fatal("joined projection functional columns", n, err)
	}
	if got, err := projected.Contains(Fact{0, 0, 1}); err != nil || !got {
		t.Fatal("joined projection", got, err)
	}
}
