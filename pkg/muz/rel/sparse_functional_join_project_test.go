package rel

import "testing"

func TestSparseFunctionalJoinProjectComposesBoundaries(t *testing.T) {
	left := newSparseFunctionalFixture(t, []uint64{4, 4}, 1, Fact{1, 2})
	right := newSparseFunctionalFixture(t, []uint64{4, 4}, 1, Fact{1, 3})
	projected, err := left.JoinProjectFunctional(right, []int{0}, []int{0}, []int{3})
	if err != nil {
		t.Fatal(err)
	}
	defer projected.Close()
	functional, _ := projected.FunctionalColumns()
	if functional != 1 {
		t.Fatalf("functional=%d", functional)
	}
	if exact, _ := projected.ContainsExact(Fact{1, 1, 2}); !exact {
		t.Fatal("projected joined fact missing")
	}
}
