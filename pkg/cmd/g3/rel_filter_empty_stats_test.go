package main

import "testing"

// AUDIT.md muz_rel_03-F2: an empty Sparse filter_interpreted result is
// made empty (null register), so the following union is skipped.
func TestRelFilterEmptySparseStatistics(t *testing.T) {
	want := ":dl.filter 1\n:dl.joins 3\n:dl.project_rename 1\n:dl.unary_singleton 4\n:dl.union 4"
	for _, options := range [][]string{nil, {"fixedpoint.datalog.default_table=hashtable"}, {"fixedpoint.datalog.default_table_checked=true"}} {
		if got := datalogStatisticsLines(t, "rel_filter_empty_sparse_stats.smt2", options...); got != want {
			t.Fatalf("%v: dl statistics\n%s\nwant\n%s", options, got, want)
		}
	}
}
