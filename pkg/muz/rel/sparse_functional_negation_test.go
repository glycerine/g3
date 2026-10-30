package rel

import (
	"errors"
	"testing"
)

func TestSparseFunctionalNegationUsesOnlyNonFunctionalIndexes(t *testing.T) {
	target := newSparseFunctionalFixture(t, []uint64{4, 8}, 1, Fact{1, 2}, Fact{2, 3})
	negative := newSparseFunctionalFixture(t, []uint64{4, 8}, 1, Fact{1, 7})
	removed, err := target.FilterNegationFunctional(negative, []int{0}, []int{0})
	if err != nil || removed != 1 {
		t.Fatalf("removed=%d err=%v", removed, err)
	}
	if exact, _ := target.ContainsExact(Fact{2, 3}); !exact {
		t.Fatal("unmatched row removed")
	}
	if _, err := target.FilterNegationFunctional(negative, []int{1}, []int{1}); !errors.Is(err, ErrTable) {
		t.Fatalf("functional index=%v", err)
	}
}
