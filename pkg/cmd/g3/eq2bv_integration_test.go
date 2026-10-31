package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSMTLibRunnerEq2BVTactic(t *testing.T) {
	script := `(set-logic QF_LIA)
(set-option :produce-models true)
(declare-const x Int)
(assert (or (= x 1) (= x 3)))
(assert (not (= x 1)))
(check-sat-using (then eq2bv qfbv))
(get-value (x))
(push)
(assert (= x 1))
(check-sat-using (then eq2bv qfbv))
(pop)
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.Join(strings.Fields(out.String()), " "); got != "sat ((x 3)) unsat" {
		t.Fatalf("eq2bv result: %s", got)
	}
}
