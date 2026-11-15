package transforms

import (
	"reflect"
	"testing"
)

// TestSubsumptionCheckerOrderMatchesStdSort: transform_rules sorts the
// rules with std::sort and rule_size_comparator
// (dl_mk_subsumption_checker.cpp:192). For 20 rules with tail sizes cycling
// over {1,0,2}, g++ 13.3's std::sort leaves the positions below (recorded
// from a small C++ program with the same comparator); a stable sort would
// keep every tie in source order (AUDIT.md muz_transforms_02-F5).
func TestSubsumptionCheckerOrderMatchesStdSort(t *testing.T) {
	sizes := make([]int, 20)
	for i := range sizes {
		sizes[i] = []int{1, 0, 2}[i%3]
	}
	got := subsumptionRuleOrder(sizes)
	want := []int{10, 19, 16, 13, 7, 4, 1, 6, 9, 0, 12, 3, 15, 18, 5, 8, 11, 14, 2, 17}
	if !reflect.DeepEqual(got, want) {
		t.Fatal(got, want)
	}
	// Up to 16 rules std::sort is an insertion sort: ties keep source order.
	if got := subsumptionRuleOrder([]int{2, 1, 1, 0, 2, 0}); !reflect.DeepEqual(got, []int{3, 5, 1, 2, 0, 4}) {
		t.Fatal("small", got)
	}
}
