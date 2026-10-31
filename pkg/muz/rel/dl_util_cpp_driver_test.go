package rel

// Source: src/test/dl_util.cpp dl_util_cycle_from_permutation.

import (
	"reflect"
	"testing"
)

func TestDatalogUtilitySourcePermutationCycles(t *testing.T) {
	for _, tc := range []struct{ input, want []uint32 }{
		{[]uint32{0, 1, 4, 3, 2, 5, 6, 7}, []uint32{2, 4}},
		{[]uint32{1, 2, 3, 4, 5, 6, 7, 0}, []uint32{0, 1, 2, 3, 4, 5, 6, 7}},
	} {
		input := append([]uint32(nil), tc.input...)
		cycle, found := removePermutationCycle(input)
		if !found || !reflect.DeepEqual(cycle, tc.want) {
			t.Fatal("source cycle", cycle, found, tc.want)
		}
		for i, v := range input {
			if v != uint32(i) {
				t.Fatal("source cycle not removed", input)
			}
		}
		if _, found = removePermutationCycle(input); found {
			t.Fatal("identity permutation has a cycle")
		}
	}
	input := []uint32{1, 0, 3, 2}
	for _, want := range [][]uint32{{0, 1}, {2, 3}} {
		cycle, found := removePermutationCycle(input)
		if !found || !reflect.DeepEqual(cycle, want) {
			t.Fatal("multiple cycles", cycle, found, want)
		}
	}
	if _, found := removePermutationCycle(input); found {
		t.Fatal("multiple cycles not fully removed")
	}
}
