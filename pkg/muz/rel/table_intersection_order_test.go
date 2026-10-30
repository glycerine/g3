package rel

import (
	"errors"
	"testing"
)

func TestHashTableIntersectionPreservesOrderAndValidatesColumns(t *testing.T) {
	target := intersectionTestTable(t, []uint64{5}, Fact{4}, Fact{1}, Fact{3}, Fact{2})
	source := intersectionTestTable(t, []uint64{5}, Fact{3}, Fact{4}, Fact{2})
	if err := target.FilterByIntersection(source, []int{0}, []int{0}); err != nil {
		t.Fatal(err)
	}
	intersectionWantFacts(t, target, Fact{4}, Fact{3}, Fact{2})
	wrong := intersectionTestTable(t, []uint64{4}, Fact{0})
	if err := target.FilterByIntersection(wrong, []int{0}, []int{0}); !errors.Is(err, ErrTable) {
		t.Fatal("domain mismatch", err)
	}
	if err := target.FilterByIntersection(source, []int{1}, []int{0}); !errors.Is(err, ErrTable) {
		t.Fatal("column range", err)
	}
}
