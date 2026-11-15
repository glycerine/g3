package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// Nonlinear BMC runs the rule transformations once per query, so -v:1
// prints each (transform ...) line once, as z3 does. g3 used to rerun the
// query with a forced proof trace and printed them twice. Expected text from
// the fork with the nonlinear BMC fix (z3 4.7.1 segfaults); timings elided.
func TestBMCNonlinearVerboseMatchesFork(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("test_vectors", "z3_bug_bmc_nonlinear_horn.v1"))
	if err != nil {
		t.Fatal(err)
	}
	var out, diagnostics bytes.Buffer
	args := []string{"-v:1", filepath.Join("test_vectors", "z3_bug_bmc_nonlinear_horn.smt2")}
	if err := runArgumentsWithDiagnostics(args, bytes.NewReader(nil), &out, &diagnostics); err != nil {
		t.Fatal(err, out.String())
	}
	got := regexp.MustCompile(`(?m) [0-9.]+s\)$`).ReplaceAllString(diagnostics.String(), " s)")
	if got != string(want) || out.String() != "unsat\n" {
		t.Fatalf("verbose\n%s\nwant\n%s\noutput\n%s", got, want, out.String())
	}
}
