package rel

import "testing"

func TestSparseJoinProjectPreservesFunctionalColumnsWhenEquivalentKeySurvives(t *testing.T) {
	left := newSparseFunctionalFixture(t, []uint64{4, 4}, 1, Fact{1, 2})
	right := newSparseFunctionalFixture(t, []uint64{4, 4}, 1, Fact{1, 3})
	result, err := left.JoinProjectFunctionalPreserving(right, []int{0}, []int{0}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	functional, _ := result.FunctionalColumns()
	if functional != 2 {
		t.Fatalf("functional=%d", functional)
	}
	if exact, _ := result.ContainsExact(Fact{1, 2, 3}); !exact {
		t.Fatal("preserved-key result missing")
	}
}
