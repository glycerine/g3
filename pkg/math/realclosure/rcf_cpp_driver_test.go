package realclosure

// Source: src/test/rcf.cpp matrix solve, tensor and independent-row examples.

import (
	"reflect"
	"testing"
)

func TestRealClosureSourceMatrixExamples(t *testing.T) {
	for _, tc := range []struct {
		matrix      []int64
		right, want []int
		solved      bool
	}{
		{[]int64{0, 1, 1, 1, 0, 1, 1, 1, -1}, []int{10, 4, -4}, []int{-2, 4, 6}, true},
		{[]int64{1, 1, 1, 0, 1, 1, 0, 1, 1}, []int{3, 2, 2}, nil, false},
	} {
		var a signMatrix
		a.make(3, 3)
		a.setSmall(tc.matrix...)
		got, ok := a.solveInt(tc.right)
		if ok != tc.solved || ok && !reflect.DeepEqual(got, tc.want) {
			t.Fatal("source matrix solve", got, ok, tc.want, tc.solved)
		}
	}
	for _, tc := range []struct {
		values []int64
		want   []int
	}{
		{[]int64{1, 1, 1, 0, 1, 1, 0, 1, 1, 1, 1, 1, 1, 0, -1}, []int{0, 1, 4}},
		{[]int64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, -1}, []int{0, 4}},
		{[]int64{1, 1, 1, 1, 1, 1, 1, 0, 1, 2, 1, 2, 3, 1, 3}, []int{0, 2}},
	} {
		var a, selected signMatrix
		a.make(5, 3)
		a.setSmall(tc.values...)
		if got := selected.independentRows(&a); !reflect.DeepEqual(got, tc.want) {
			t.Fatal("source independent rows", got, tc.want)
		}
	}
	var a, product, selected signMatrix
	a.make(3, 3)
	a.setSmall(1, 1, 1, 0, 1, -1, 0, 1, 1)
	if got, ok := a.solveInt([]int{10, -2, 8}); !ok || !reflect.DeepEqual(got, []int{2, 3, 5}) {
		t.Fatal("source displayed solve", got, ok)
	}
	product.tensorProduct(&a, &a)
	selected.filterColumns(&product, []int{1, 3, 7, 8})
	for row := 0; row < 9; row++ {
		for col, original := range []int{1, 3, 7, 8} {
			if selected.at(row, col).Cmp(product.at(row, original)) != 0 {
				t.Fatal("source tensor column", row, col)
			}
		}
	}
}
