package rel

import (
	"errors"
	"testing"
)

func TestSparseNegatedJoinPlanValidatesMappingsAndDomains(t *testing.T) {
	target := newSparseFunctionalFixture(t, []uint64{4}, 0)
	first := newSparseFunctionalFixture(t, []uint64{4}, 0)
	second := newSparseFunctionalFixture(t, []uint64{8}, 0)
	if _, err := NewSparseNegatedJoinPlan(target, first, second, []int{0}, nil, nil, nil); !errors.Is(err, ErrTable) {
		t.Fatalf("mapping length=%v", err)
	}
	if _, err := NewSparseNegatedJoinPlan(target, first, second, []int{0}, []int{1}, nil, nil); !errors.Is(err, ErrTable) {
		t.Fatalf("mapping domain=%v", err)
	}
	if _, err := NewSparseNegatedJoinPlan(target, first, second, nil, nil, []int{0}, []int{0}); !errors.Is(err, ErrTable) {
		t.Fatalf("join domain=%v", err)
	}
	if _, err := NewSparseNegatedJoinPlan(target, first, first, nil, nil, nil, nil); !errors.Is(err, ErrTable) {
		t.Fatalf("aliased sources=%v", err)
	}
}
