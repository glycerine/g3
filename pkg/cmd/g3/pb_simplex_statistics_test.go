package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// pbSimplexStatisticLine keeps the answers and the simplex statistics
// (sparse_matrix add rows and the simplex pivot/check/infeasible counters).
var pbSimplexStatisticLine = regexp.MustCompile(`^(sat|unsat|unknown)$|^\s*\(?:simplex-[a-z-]+\s+[0-9]+\)?$`)

func pbSimplexStatistics(output string) string {
	var b strings.Builder
	for _, line := range strings.Split(output, "\n") {
		if pbSimplexStatisticLine.MatchString(line) {
			b.WriteString(strings.Join(strings.Fields(strings.Trim(line, "()")), " "))
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// AUDIT.md smtcore_12-F4 and math_misc_02-F3: with -st, theory_pb's simplex
// statistics match build/z3 once select_pivot_core breaks ties with
// m_random and sparse_matrix reuses freed row and column slots LIFO. The
// expected texts are what build/z3 -st prints for these vectors.
func TestPBSimplexStatisticsMatchSource(t *testing.T) {
	cases := map[string]string{
		"pb_simplex_pivot_stats.smt2":    "sat\n:simplex-num-checks 2\n:simplex-num-pivots 2\n",
		"pb_simplex_add_rows_stats.smt2": "unsat\n:simplex-add-rows 6\n:simplex-num-checks 4\n:simplex-num-infeasible 2\n:simplex-num-pivots 7\n",
		"pb_simplex_tie_stats.smt2":      "unsat\n:simplex-add-rows 117\n:simplex-num-checks 6\n:simplex-num-infeasible 1\n:simplex-num-pivots 28\n",
		"pb_simplex_scopes_stats.smt2":   "sat\nsat\nsat\nsat\nsat\n:simplex-add-rows 143\n:simplex-num-checks 52\n:simplex-num-infeasible 8\n:simplex-num-pivots 43\n",
	}
	reference := os.Getenv("G3_CPP_REFERENCE")
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("test_vectors", name)
			var out bytes.Buffer
			if err := runArguments([]string{"-st", path}, strings.NewReader(""), &out); err != nil {
				t.Fatal(err, out.String())
			}
			if got := pbSimplexStatistics(out.String()); got != want {
				t.Fatalf("simplex statistics\n got: %q\nwant: %q\nfull output:\n%s", got, want, out.String())
			}
			if reference != "" {
				native, err := exec.Command(reference, "-st", path).Output()
				if err != nil {
					t.Fatal("C++ reference", err, string(native))
				}
				if got := pbSimplexStatistics(string(native)); got != want {
					t.Fatalf("C++ reference simplex statistics\n got: %q\nwant: %q", got, want)
				}
			}
		})
	}
}
