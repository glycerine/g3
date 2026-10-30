package rel

import (
	"errors"
	"testing"
)

func TestSparseFunctionalOperationsRejectInvalidColumnsAndFacts(t *testing.T) {
	left := newSparseFunctionalFixture(t, []uint64{4, 4}, 1, Fact{1, 2})
	right := newSparseFunctionalFixture(t, []uint64{8, 4}, 1, Fact{2, 3})
	if _, err := left.JoinFunctional(right, []int{0}, []int{0}); !errors.Is(err, ErrTable) {
		t.Fatalf("join domain=%v", err)
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
