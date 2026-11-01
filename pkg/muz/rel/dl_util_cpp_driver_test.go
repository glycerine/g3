package rel

// Source: src/test/dl_util.cpp dl_util_two_array_sort and
// dl_util_cycle_from_permutation.

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

func TestDatalogUtilitySourcePairedSort(t *testing.T) {
	const n = 97
	pairs := make([][2]int, n)
	for i := range pairs {
		key := (i * 30) % n
		pairs[i] = [2]int{key, key + 3}
	}
	sortPairedColumnIndices(pairs)
	for i, pair := range pairs {
		if pair != [2]int{i, i + 3} {
			t.Fatalf("sorted pair %d=%v, want [%d %d]", i, pair, i, i+3)
		}
	}
	for _, tc := range []struct {
		input [][2]int
		want  [][2]int
	}{
		{nil, nil},
		{[][2]int{{4, 9}}, [][2]int{{4, 9}}},
		{[][2]int{{1, 8}, {0, 7}}, [][2]int{{0, 7}, {1, 8}}},
	} {
		got := append([][2]int(nil), tc.input...)
		sortPairedColumnIndices(got)
		if len(got) != len(tc.want) {
			t.Fatalf("sorted %v has length %d, want %d", tc.input, len(got), len(tc.want))
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("sorted %v=%v, want %v", tc.input, got, tc.want)
			}
		}
	}
}
