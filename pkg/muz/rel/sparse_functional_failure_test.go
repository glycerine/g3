package rel

import (
	"errors"
	"testing"
)

func TestSparseFunctionalOperationsRejectInvalidColumnsAndFacts(t *testing.T) {
	left := newSparseFunctionalFixture(t, []uint64{4, 4}, 1, Fact{1, 2})
	right := newSparseFunctionalFixture(t, []uint64{8, 4}, 1, Fact{1, 3})
	joined, err := left.JoinFunctional(right, []int{0}, []int{0})
	if err != nil {
		t.Fatalf("join across different finite domain bounds: %v", err)
	}
	if count, err := joined.Len(); err != nil || count != 1 {
		t.Fatalf("joined rows=%d, err=%v", count, err)
	}
	if err := joined.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := left.ProjectFunctional([]int{1, 0}); !errors.Is(err, ErrTable) {
		t.Fatalf("project order=%v", err)
	}
	if _, err := left.FetchFact(Fact{1}); !errors.Is(err, ErrTable) {
		t.Fatalf("fetch arity=%v", err)
	}
	if err := left.EnsureFact(Fact{1, 9}); !errors.Is(err, ErrTable) {
		t.Fatalf("ensure domain=%v", err)
	}
}
