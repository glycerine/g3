package main

import (
	"strings"
	"testing"
	"time"
)

// AUDIT.md muz_fp_04-F1: pdr::dl_interface::updt_params, run by every
// (query ...), replaces the pdr::context, so each query's statistics block
// reports only that query (z3 prints the same counters twice).
func TestPDRStatisticsPerQuery(t *testing.T) {
	const script = `(set-option :fixedpoint.engine pdr)
(set-option :fixedpoint.print_statistics true)
(declare-rel inv (Int))
(declare-rel err ())
(declare-var x Int)
(rule (=> (= x 0) (inv x)))
(rule (=> (and (inv x) (< x 5)) (inv (+ x 1))))
(rule (=> (and (inv x) (> x 5)) err))
(query err)
(query err)
`
	out := runScriptWithin(t, script, 2*time.Minute)
	for _, want := range []string{" :PDR-num-unfoldings  2\n", " :conflicts           6\n", " :dl-conflicts        4\n", " :num-checks          9\n"} {
		if got := strings.Count(out, want); got != 2 {
			t.Fatalf("want %q in both statistics blocks, found %d times:\n%s", want, got, out)
		}
	}
}
