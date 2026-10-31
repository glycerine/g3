package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSMTLibRunnerRecover01Tactic(t *testing.T) {
	script := `(set-logic QF_LIA)
(set-option :produce-models true)
(declare-const p Bool)
(declare-const x Int)
(assert (or p (= x 0)))
(assert (or (not p) (= x 3)))
(assert p)
(check-sat-using (then recover-01 smt))
(get-value (p x))
(push)
(assert (= x 0))
(check-sat-using (then recover-01 smt))
(pop)
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.Join(strings.Fields(out.String()), " "); got != "sat ((p true) (x 3)) unsat" {
		t.Fatalf("recover-01 result: %s", got)
	}
}
