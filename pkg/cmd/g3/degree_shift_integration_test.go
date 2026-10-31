package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSMTLibRunnerDegreeShiftTactic(t *testing.T) {
	script := `(set-logic QF_NRA)
(set-option :produce-models true)
(declare-const x Real)
(assert (= (* x x) 4))
(check-sat-using (then degree-shift nlsat))
(get-value ((* x x)))
(push)
(assert (= (* x x) (- 1)))
(check-sat-using (then degree-shift nlsat))
(pop)
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.Join(strings.Fields(out.String()), " "); got != "sat (((* x x) 4.0)) unsat" {
		t.Fatalf("degree-shift result: %s", got)
	}
}
