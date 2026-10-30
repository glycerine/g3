package rel

import (
	"errors"
	"testing"
)

func TestSparseIndexedNegationValidatesMappingsAndSchemas(t *testing.T) {
	target := newSparseFunctionalFixture(t, []uint64{4, 8}, 1)
	negative := newSparseFunctionalFixture(t, []uint64{4, 16}, 1)
	cases := []struct{ target, negative []int }{
		{[]int{0}, nil}, {[]int{0}, []int{1}}, {[]int{1}, []int{0}}, {[]int{2}, []int{0}},
	}
	for _, tc := range cases {
		if _, err := NewSparseIndexedNegationPlan(target, negative, tc.target, tc.negative); !errors.Is(err, ErrTable) {
			t.Fatalf("case=%+v err=%v", tc, err)
		}
	}
	plan, err := NewSparseIndexedNegationPlan(target, negative, []int{0}, []int{0})
	if err != nil {
		t.Fatal(err)
	}
	wrong := newSparseFunctionalFixture(t, []uint64{5, 8}, 1)
	if _, err := plan.Execute(wrong, negative); !errors.Is(err, ErrTable) {
		t.Fatalf("schema drift: %v", err)
	}
	if _, err := (*SparseIndexedNegationPlan)(nil).Execute(target, negative); !errors.Is(err, ErrTable) {
		t.Fatalf("nil plan: %v", err)
	}
}

func TestSparseIndexedNegationPlanOwnsColumnsAndClone(t *testing.T) {
	target := newSparseFunctionalFixture(t, []uint64{4}, 0)
	negative := newSparseFunctionalFixture(t, []uint64{4}, 0)
	targetColumns, negativeColumns := []int{0}, []int{0}
	plan, err := NewSparseIndexedNegationPlan(target, negative, targetColumns, negativeColumns)
	if err != nil {
		t.Fatal(err)
	}
	targetColumns[0], negativeColumns[0] = 3, 3
	left, right := plan.Columns()
	left[0], right[0] = 2, 2
	cloneLeft, cloneRight := plan.Clone().Columns()
	if cloneLeft[0] != 0 || cloneRight[0] != 0 {
		t.Fatalf("clone columns=%v/%v", cloneLeft, cloneRight)
	}
}
