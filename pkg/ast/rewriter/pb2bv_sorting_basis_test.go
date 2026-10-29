package rewriter

// Source: src/ast/rewriter/pb2bv_rewriter.cpp recursive prime basis search.

import "testing"

func TestPB2BVSortingBasisSourceCostAndDeterminism(t *testing.T) {
	for _, coefficients := range [][]uint32{{1, 3, 5}, {2, 3, 7, 11}, {0, 0, 1}, {4, 8, 12}} {
		first, ok := pb2BVCreateSortingBasis(coefficients)
		if !ok || len(first) == 0 || first[len(first)-1] > uint32(20*len(first)) {
			t.Fatal("source basis refusal", coefficients, first, ok)
		}
		second, ok := pb2BVCreateSortingBasis(coefficients)
		if !ok || len(first) != len(second) {
			t.Fatal("source basis instability", first, second)
		}
		for i := range first {
			if first[i] != second[i] {
				t.Fatal("source prime traversal order", first, second)
			}
		}
	}
}
