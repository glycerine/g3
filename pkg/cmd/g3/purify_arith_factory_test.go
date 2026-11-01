package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSMTLibRunnerPurifyArithTactic(t *testing.T) {
	script := `(set-logic QF_NIA)
(set-option :produce-models true)
(declare-const x Int)
(assert (= (rem x 3) 1))
(assert (= x 4))
(check-sat-using (then purify-arith smt))
(get-value (x))
(push)
(assert (= x 5))
(check-sat-using (then purify-arith smt))
(pop)
`
	var output bytes.Buffer
	if err := run(strings.NewReader(script), &output); err != nil {
		t.Fatal(err, output.String())
	}
	if got := strings.Join(strings.Fields(output.String()), " "); got != "sat ((x 4)) unsat" {
		t.Fatal("purify-arith result", got)
	}
}
