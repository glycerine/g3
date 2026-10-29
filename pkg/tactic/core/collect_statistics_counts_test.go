package core

import "testing"

func TestCollectStatisticsUninterpretedCountsAndSharedTraversal(t *testing.T) {
	f := newCollectStatisticsFixture(t)
	r := f.runner(t)
	term := f.app(t, f.f, f.a)
	applyStatistics(t, r, f.goal(t, term, term))
	s, e := r.Snapshot()
	simplifyMust(t, e)
	want := map[string]uint64{"function-applications": 4, "uninterpreted-sorts": 1, "uninterpreted-sort-occurrences": 3, "uninterpreted-constants": 1, "uninterpreted-functions": 1, "uninterpreted-function-occurrences": 1}
	for k, v := range want {
		if s[k] != v {
			t.Fatal(k, s[k], v, s)
		}
	}
}

func TestCollectStatisticsDistinctConstantsShareSortOnly(t *testing.T) {
	f := newCollectStatisticsFixture(t)
	r := f.runner(t)
	applyStatistics(t, r, f.goal(t, f.a, f.b))
	s, e := r.Snapshot()
	simplifyMust(t, e)
	if s["uninterpreted-constants"] != 2 || s["uninterpreted-sorts"] != 1 || s["uninterpreted-sort-occurrences"] != 2 {
		t.Fatal(s)
	}
}
