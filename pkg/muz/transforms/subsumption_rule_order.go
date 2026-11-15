package transforms

// Source: src/muz/transforms/dl_mk_subsumption_checker.cpp:175-177
// (rule_size_comparator) and :192 (std::sort in transform_rules).

import "github.com/glycerine/g3/pkg/util/sorting"

// =========================================================================
// !!! USER-APPROVED libstdc++ std::sort CALL SITE (owner, 2026-11-14) !!!
// =========================================================================
//
// AUDIT.md muz_transforms_02-F5. mk_subsumption_checker::transform_rules
// sorts orig_rules with
//
//	std::sort(orig_rules.c_ptr(), orig_rules.c_ptr() + orig_rules.size(), rule_size_comparator);
//
// (dl_mk_subsumption_checker.cpp:192). rule_size_comparator compares tail
// sizes only, so with more than 16 rules libstdc++'s introsort permutes
// equal-size rules, which decides the output rule order and which duplicate
// survives subs_index.is_subsumed. It must use
// pkg/util/sorting/libstdcxx_sort.go, never sort.Slice/sort.SliceStable.

// rulesByTailSize views orig_rules through positions into the source rule
// list; sorting positions with the same comparator is sorting the rules.
type rulesByTailSize struct {
	order []int
	sizes []int
}

func (r rulesByTailSize) Len() int           { return len(r.order) }
func (r rulesByTailSize) Swap(i, j int)      { r.order[i], r.order[j] = r.order[j], r.order[i] }
func (r rulesByTailSize) Less(i, j int) bool { return r.sizes[r.order[i]] < r.sizes[r.order[j]] }

// subsumptionRuleOrder returns the positions of the source rules in the
// order std::sort(..., rule_size_comparator) leaves them.
func subsumptionRuleOrder(sizes []int) []int {
	order := make([]int, len(sizes))
	for position := range order {
		order[position] = position
	}
	sorting.LibstdcxxSort(rulesByTailSize{order: order, sizes: sizes})
	return order
}
