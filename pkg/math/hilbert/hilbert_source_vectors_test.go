package hilbert

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestAdditionalSourceHilbertVectors(t *testing.T) {
	tests := []struct {
		name   string
		rows   [][]int64
		bounds []int64
		equal  []bool
	}{
		{"example-10", [][]int64{{-23, 12, 9}, {-1, 8, 8}}, []int64{0, 0}, []bool{true, true}},
		{"single-inequality", [][]int64{{3, 2, -1, -2}}, []int64{0}, []bool{false}},
		{"affine-box", [][]int64{{1, 0}, {0, 1}}, []int64{1, 1}, []bool{false, false}},
		{"opposite-equalities", [][]int64{{1, 0}, {-1, 0}, {0, 2}, {0, -2}}, []int64{0, 0, 0, 0}, []bool{true, true, true, true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			b := New(nil)
			for index, row := range test.rows {
				var err error
				if test.equal[index] {
					err = b.AddEQ(rationals(row...), rational(test.bounds[index]))
				} else {
					err = b.AddLE(rationals(row...), rational(test.bounds[index]))
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			result, err := b.Saturate()
			if err != nil || result != lbool.True {
				t.Fatalf("Saturate = %v, %v", result, err)
			}
			validateBasis(t, b)
		})
	}
}
