package rel

import (
	"errors"
	"testing"
)

func TestSparseIndexedJoinValidatesMappingsProjectionAndSchemas(t *testing.T) {
	left := newSparseFunctionalFixture(t, []uint64{4, 8}, 1)
	right := newSparseFunctionalFixture(t, []uint64{4, 16}, 1)
	cases := []struct {
		left, right, removed []int
	}{
		{[]int{0}, nil, nil},
		{[]int{0}, []int{1}, nil},
		{[]int{1}, []int{0}, nil},
		{nil, nil, []int{0, 1, 2, 3}},
		{nil, nil, []int{2, 1}},
	}
	for _, tc := range cases {
		if _, err := NewSparseIndexedJoinPlan(left, right, tc.left, tc.right, tc.removed); !errors.Is(err, ErrTable) {
			t.Fatalf("case %+v: %v", tc, err)
		}
	}
	plan, err := NewSparseIndexedJoinPlan(left, right, []int{0}, []int{0}, nil)
	if err != nil {
		t.Fatal(err)
	}
	wrong := newSparseFunctionalFixture(t, []uint64{5, 8}, 1)
	if _, _, err := plan.Execute(wrong, right); !errors.Is(err, ErrTable) {
		t.Fatalf("schema drift: %v", err)
	}
	if _, _, err := (*SparseIndexedJoinPlan)(nil).Execute(left, right); !errors.Is(err, ErrTable) {
		t.Fatalf("nil plan: %v", err)
	}
}

func TestSparseIndexedJoinPlanOwnsColumnsAndClone(t *testing.T) {
	left := newSparseFunctionalFixture(t, []uint64{4}, 0)
	right := newSparseFunctionalFixture(t, []uint64{4}, 0)
	lc, rc, removed := []int{0}, []int{0}, []int{1}
	plan, err := NewSparseIndexedJoinPlan(left, right, lc, rc, removed)
	if err != nil {
		t.Fatal(err)
	}
	lc[0], rc[0], removed[0] = 9, 9, 9
	a, b, c := plan.Columns()
	a[0], b[0], c[0] = 8, 8, 8
	clone := plan.Clone()
	ca, cb, cc := clone.Columns()
	if ca[0] != 0 || cb[0] != 0 || cc[0] != 1 {
		t.Fatalf("clone columns=%v/%v/%v", ca, cb, cc)
	}
}
