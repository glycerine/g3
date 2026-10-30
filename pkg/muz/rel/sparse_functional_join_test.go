package rel

import "testing"

func TestSparseFunctionalJoinUsesSourceColumnOrder(t *testing.T) {
	left := newSparseFunctionalFixture(t, []uint64{4, 4}, 1, Fact{1, 2})
	right := newSparseFunctionalFixture(t, []uint64{4, 4}, 1, Fact{2, 3}, Fact{0, 1})
	joined, err := left.JoinFunctional(right, []int{1}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	defer joined.Close()
	functional, _ := joined.FunctionalColumns()
	if functional != 2 {
		t.Fatalf("functional=%d", functional)
	}
	if exact, err := joined.ContainsExact(Fact{1, 2, 2, 3}); err != nil || !exact {
		t.Fatalf("joined exact=%v err=%v", exact, err)
	}
}
