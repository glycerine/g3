package main

import (
	"bytes"
	"path/filepath"
	"regexp"
	"testing"
)

// TestCheckRelationRouteRlimit: AUDIT.md FUZZ-F5 (relation plugins).
// check_relation over bound_relation, interval_relation, karr_relation and
// the tr_* table_relation plugins runs on the relation plugins; the
// rlimit-count equals the fixed fork's. A predicate's relation is created
// where relation_manager::get_relation creates it (after saturate for a
// queried predicate without rules, at the instr_io load of a rule's
// predicate), so a failing mk_empty check stops at the same point.
func TestCheckRelationRouteRlimit(t *testing.T) {
	rlimit := regexp.MustCompile(`:rlimit-count +([0-9]+)`)
	for name, want := range map[string]string{
		"plugins4_check_bound_empty.smt2":      "29",
		"plugins4_check_interval_rule.smt2":    "53",
		"plugins4_check_karr_no_xform.smt2":    "41",
		"plugins4_check_karr_xform_bool.smt2":  "95",
		"plugins4_check_tr_sparse.smt2":        "3327",
		"plugins4_check_tr_hashtable.smt2":     "2032",
		"plugins4_check_hashtable_nobase.smt2": "76",
		"plugins4_check_doc_mixed_join.smt2":   "114",
		"plugins4_check_bound_mixed.smt2":      "83",
		"plugins4_check_tr_sparse_mixed.smt2":  "1209",
		"plugins4_doc_table_full_columns.smt2": "47",
	} {
		var out, diagnostics bytes.Buffer
		args := []string{"-st", filepath.Join("test_vectors", name)}
		if err := runArgumentsWithDiagnostics(args, bytes.NewReader(nil), &out, &diagnostics); err != nil {
			t.Fatal(name, err, out.String())
		}
		got := rlimit.FindStringSubmatch(out.String())
		if got == nil || got[1] != want {
			t.Fatalf("%s: rlimit-count %v, want %s\n%s", name, got, want, out.String())
		}
	}
}
