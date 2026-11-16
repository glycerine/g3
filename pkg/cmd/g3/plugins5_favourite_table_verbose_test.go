//go:build g3trace

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPlugins5FavouriteTableVerbose: AUDIT.md FUZZ-F5 (relation plugins).
// With a tr_hashtable / tr_bitvector favourite the -v:2 instruction lines
// (register loads, joins, unions, "new type returned" where a bitvector
// join yields a sparse table, recycle) equal build/z3's; the expected lines
// are the fixed fork's -v:2 stderr without the "(transform ...)" and
// "...<time>" progress lines.
func TestPlugins5FavouriteTableVerbose(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string][]string{
		"plugins5_tr_hashtable_favourite.smt2": {
			"load e into 0",
			"load t into 1",
			"mk_unary_singleton into 2 sort:(_ BitVec 3) val:1",
			"mk_unary_singleton into 3 sort:(_ BitVec 3) val:2",
			"join 2() and 3() into 4",
			"union 4 into 0",
			"mk_unary_singleton into 5 sort:(_ BitVec 3) val:3",
			"join 3() and 5() into 6",
			"union 6 into 0",
			"mk_unary_singleton into 7 sort:(_ BitVec 3) val:4",
			"join 5() and 7() into 8",
			"union 8 into 0",
			"mk_unary_singleton into 9 sort:(_ BitVec 3) val:6",
			"join 9() and 2() into 10",
			"union 10 into 0",
			"mark_saturated e",
			"union 0 into 1",
			"clone 1 into 11",
			"while(11)",
			"    join_project 0:2-4(0) and 11:2-4(1) into 13 removing columns (0,3)",
			"    rename 13 into 13 with cycle (0,1)",
			"    union 13 into 1 with delta 12",
			"    dealloc 13",
			"    move 12 into 11",
			"join_project 0:2-4(0) and 11:2-4(1) into 13 removing columns (0,3)",
			"rename 13 into 13 with cycle (0,1)",
			"union 13 into 1 with delta 12",
			"move 12 into 11",
			"join_project 0:2-4(0) and 11:2-3(1) into 13 removing columns (0,3)",
			"rename 13 into 13 with cycle (0,1)",
			"union 13 into 1 with delta 12",
			"move 12 into 11",
			"join_project 0:2-4(0) and 11:2-2(1) into 13 removing columns (0,3)",
			"rename 13 into 13 with cycle (0,1)",
			"union 13 into 1 with delta 12",
			"move 12 into 11",
			"join_project 0:2-4(0) and 11:2-1(1) into 13 removing columns (0,3)",
			"mark_saturated t",
			"store 0 into e",
			"store 1 into t",
		},
		"plugins5_tr_bitvector_favourite.smt2": {
			"load e into 0",
			"load t into 1",
			"mk_unary_singleton into 2 sort:(_ BitVec 20) val:1",
			"mk_unary_singleton into 3 sort:(_ BitVec 20) val:2",
			"join 2() and 3() into 4",
			"new type returned",
			"union 4 into 0",
			"mk_unary_singleton into 5 sort:(_ BitVec 20) val:3",
			"join 3() and 5() into 6",
			"new type returned",
			"union 6 into 0",
			"mk_unary_singleton into 7 sort:(_ BitVec 20) val:9",
			"join 7() and 2() into 8",
			"new type returned",
			"union 8 into 0",
			"mark_saturated e",
			"union 0 into 1",
			"clone 1 into 9",
			"while(9)",
			"    join_project 0:2-3(0) and 9:2-3(1) into 11 removing columns (0,3)",
			"    rename 11 into 11 with cycle (0,1)",
			"    union 11 into 1 with delta 10",
			"    dealloc 11",
			"    move 10 into 9",
			"join_project 0:2-3(0) and 9:2-3(1) into 11 removing columns (0,3)",
			"rename 11 into 11 with cycle (0,1)",
			"union 11 into 1 with delta 10",
			"move 10 into 9",
			"join_project 0:2-3(0) and 9:2-2(1) into 11 removing columns (0,3)",
			"rename 11 into 11 with cycle (0,1)",
			"union 11 into 1 with delta 10",
			"move 10 into 9",
			"join_project 0:2-3(0) and 9:2-1(1) into 11 removing columns (0,3)",
			"mark_saturated t",
			"store 1 into t",
			"store 0 into e",
			"load w into 0",
			"load n into 1",
			"load m into 2",
			"mk_unary_singleton into 3 sort:(_ BitVec 3) val:1",
			"union 3 into 0",
			"mk_unary_singleton into 4 sort:(_ BitVec 3) val:6",
			"union 4 into 0",
			"mark_saturated w",
			"filter_interpreted_and_project 0 into 5 using (not (= (:var 0) #b001)) deleting columns (0)",
			"union 5 into 1",
			"mark_saturated n",
			"clone 0 into 6",
			"filter_by_negation on 6() with 1() as the negated table",
			"join 0() and 0() into 7",
			"mk_total into 8 sort:[(_ BitVec 3)] m",
			"join 7() and 8() into 7",
			"filter_interpreted_and_project 7 into 7 using (and (= (:var 2) (bvadd #b001 (:var 0))) (not (= (:var 1) #b001))) deleting columns (0,1)",
			"union 7 into 2",
			"mark_saturated m",
			"store 2 into m",
			"store 0 into w",
			"store 1 into n",
			"load w into 0",
			"load n into 1",
			"mk_unary_singleton into 2 sort:(_ BitVec 3) val:1",
			"union 2 into 0",
			"mk_unary_singleton into 3 sort:(_ BitVec 3) val:6",
			"union 3 into 0",
			"mark_saturated w",
			"filter_interpreted_and_project 0 into 4 using (not (= (:var 0) #b001)) deleting columns (0)",
			"union 4 into 1",
			"mark_saturated n",
			"store 0 into w",
			"store 1 into n",
		},
		"plugins5_nullary_tr_sparse.smt2": {
			"load a into 0",
			"mk_total into 1 sort:[] a",
			"union 1 into 0",
			"mark_saturated a",
			"store 0 into a",
			"load w into 0",
			"mk_unary_singleton into 1 sort:(_ BitVec 20) val:1",
			"mk_unary_singleton into 2 sort:(_ BitVec 20) val:2",
			"join 1() and 2() into 3",
			"union 3 into 0",
			"mark_saturated w",
			"store 0 into w",
		},
		"plugins5_wide_tr_bitvector.smt2": {
			"load a into 0",
			"mk_total into 1 sort:[] a",
			"union 1 into 0",
			"mark_saturated a",
			"store 0 into a",
			"load w into 0",
			"mk_unary_singleton into 1 sort:(_ BitVec 20) val:1",
			"mk_unary_singleton into 2 sort:(_ BitVec 20) val:2",
			"join 1() and 2() into 3",
			"new type returned",
			"union 3 into 0",
			"mark_saturated w",
			"store 0 into w",
		},
	} {
		log := runVerboseProgressChild(t, []string{"-v:2", filepath.Join(wd, "test_vectors", name)})
		var got []string
		for _, line := range strings.Split(log, "\n") {
			if line != "" && !strings.HasPrefix(line, "(") && !strings.Contains(line, "...") {
				got = append(got, line)
			}
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Fatalf("%s: -v:2 lines\n%s\nwant\n%s", name, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}
}
