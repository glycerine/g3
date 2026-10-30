package rel

import "testing"

func TestSparseJoinProjectClearsFunctionalColumnsWhenRowsCanMerge(t *testing.T) {
	left := newSparseFunctionalFixture(t, []uint64{4, 4}, 1, Fact{1, 2})
	right := newSparseFunctionalFixture(t, []uint64{4, 4}, 1, Fact{0, 3})
	result, err := left.JoinProjectFunctionalPreserving(right, nil, nil, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	functional, _ := result.FunctionalColumns()
	if functional != 0 {
		t.Fatalf("functional=%d", functional)
	}
	if exact, _ := result.ContainsExact(Fact{0, 2, 3}); !exact {
		t.Fatal("merged-row result missing")
	}
}
