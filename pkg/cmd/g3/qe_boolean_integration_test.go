package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSMTLibRunnerBooleanQETactic(t *testing.T) {
	script := `(set-option :produce-models true)
(declare-const r Bool)
(assert (forall ((x Bool)) (or x r)))
(assert (forall ((x Bool)) (exists ((y Bool)) (= x y))))
(check-sat-using (then qe smt))
(get-value (r))
(push)
(assert (not r))
(check-sat-using (then qe smt))
(pop)
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.Join(strings.Fields(out.String()), " "); got != "sat ((r true)) unsat" {
		t.Fatalf("Boolean QE tactic result: %s", got)
	}
}
