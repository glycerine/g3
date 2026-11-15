package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// AUDIT.md smtcore_12-F4 follow-up: theory_pb numbers simplex columns by
// bool var, so a column freed by pop keeps its value when the var is reused
// (as a 0-1 column or as a slack) and steers later pivots. The expected text
// is what build/z3 -st prints (the fork with the pb simplex fixes prints the
// same).
func TestPBSimplexPopReuseStatisticsMatchSource(t *testing.T) {
	const name = "pb_simplex_pop_reuse_stats.smt2"
	const want = "unsat\nsat\nsat\n:simplex-add-rows 4\n:simplex-num-checks 13\n:simplex-num-infeasible 1\n:simplex-num-pivots 14\n"
	path := filepath.Join("test_vectors", name)
	var out bytes.Buffer
	if err := runArguments([]string{"-st", path}, strings.NewReader(""), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := pbSimplexStatistics(out.String()); got != want {
		t.Fatalf("simplex statistics\n got: %q\nwant: %q\nfull output:\n%s", got, want, out.String())
	}
	if reference := os.Getenv("G3_CPP_REFERENCE"); reference != "" {
		native, err := exec.Command(reference, "-st", path).Output()
		if err != nil {
			t.Fatal("C++ reference", err, string(native))
		}
		if got := pbSimplexStatistics(string(native)); got != want {
			t.Fatalf("C++ reference simplex statistics\n got: %q\nwant: %q", got, want)
		}
	}
}
