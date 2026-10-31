package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSMTLibRunnerQESATTactic(t *testing.T) {
	script := `(set-logic ALL)
(push)
(assert (forall ((x Bool)) (exists ((y Bool)) (= x y))))
(check-sat-using qe-sat)
(pop)
(push)
(assert (exists ((x Bool)) (forall ((y Bool)) (= x y))))
(check-sat-using qe-sat)
(pop)
(push)
(assert (forall ((x Int)) (>= x 0)))
(check-sat-using qe-sat)
(pop)
(push)
(assert (forall ((b Bool)) b))
(check-sat-using qe-sat)
(pop)
(push)
(assert (exists ((x Int) (b Bool)) (and b (= x 3))))
(check-sat-using qe-sat)
(pop)
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.Join(strings.Fields(out.String()), " "); got != "sat unsat unsat unsat sat" {
		t.Fatalf("QE-SAT result: %s", got)
	}
}
