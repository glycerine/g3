package main

import (
	"bytes"
	"path/filepath"
	"regexp"
	"testing"
)

// AUDIT.md FUZZ-F5 (relation plugins): instr_filter_interpreted_and_project
// keeps the functor it made for a relation kind (instruction::find_fn /
// store_fn), so the sieve relation's var_subst of the condition runs once
// per instruction, not on every loop round. The rlimit counts are the fixed
// fork's -st output for these vectors.
func TestRelationFilterFnCacheRlimit(t *testing.T) {
	rlimit := regexp.MustCompile(`:rlimit-count +([0-9]+)`)
	for name, want := range map[string]string{
		"plugins3_interval_filter_gt.smt2": "78",
		"plugins3_interval_filter_ge.smt2": "66",
		"plugins3_bound_filter_gt.smt2":    "78",
		"plugins3_pentagon_filter_ge.smt2": "66",
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
