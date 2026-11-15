package main

import (
	"bytes"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

var optStatisticsVolatile = regexp.MustCompile(`(?m)^\s*\(?:(time|total-time|max-memory|memory|num-allocs|rlimit-count)\s+[0-9.]+\)?\s*$\n?`)

// AUDIT.md opt_01-F5: after a MaxSMT check-sat, m_check_sat_result is the
// opt context, whose collect_statistics reports the opt_solver's smt
// statistics and maxres-cores / maxres-correction-sets.
func TestOptStatisticsReachAllStatistics(t *testing.T) {
	script := `(declare-const x Int)
(declare-const y Int)
(declare-const a Bool)
(declare-const b Bool)
(declare-const c Bool)
(assert (<= 0 x 10))
(assert (<= 0 y 10))
(assert (=> a (> x 5)))
(assert (=> b (< x 3)))
(assert (=> c (> (+ x y) 12)))
(assert-soft a :weight 2)
(assert-soft b :weight 3)
(assert-soft c :weight 1)
(assert-soft (> y 8) :weight 1)
(check-sat)
(get-info :all-statistics)
`
	want := `sat
(:arith-assert-lower     14
 :arith-assert-upper     14
 :arith-bound-prop       1
 :arith-pivots           6
 :binary-propagations    38
 :conflicts              1
 :decisions              14
 :final-checks           6
 :maxres-cores           1
 :maxres-correction-sets 1
 :mk-bool-var            16
 :mk-clause              2
 :num-checks             6
 :propagations           46
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := optStatisticsVolatile.ReplaceAllString(out.String(), ""); got != want {
		t.Fatalf("opt statistics\n got: %q\nwant: %q", got, want)
	}
	if ref := os.Getenv("G3_CPP_REFERENCE"); ref != "" {
		cmd := exec.Command(ref, "-in")
		cmd.Stdin = strings.NewReader(script)
		native, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatal(err, string(native))
		}
		if got := optStatisticsVolatile.ReplaceAllString(string(native), ""); got != want {
			t.Fatalf("reference opt statistics\n got: %q\nwant: %q", got, want)
		}
	}
}
