package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSMTLibRunnerLIA2CardTactic(t *testing.T) {
	script := `(set-logic QF_LIA)
(declare-const x Int)
(declare-const y Int)
(assert (and (>= x 0) (<= x 1)))
(assert (and (>= y 0) (<= y 1)))
(assert (= (+ x y) 1))
(check-sat-using (then lia2card smt))
(check-sat-using (then (using-params lia2card :compile_equality true) smt))
(push)
(assert (= x 0))
(assert (= y 0))
(check-sat-using (then lia2card smt))
(check-sat-using (then (using-params lia2card :compile_equality true) smt))
(pop)
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.Join(strings.Fields(out.String()), " "); got != "sat sat unsat unsat" {
		t.Fatal("lia2card result", got)
	}
}

func TestSMTLibRunnerLIA2CardUnsatCoreIncludesBounds(t *testing.T) {
	script := `(set-logic QF_LIA)
(set-option :produce-unsat-cores true)
(declare-const x Int)
(assert (! (>= x 0) :named lo))
(assert (! (<= x 1) :named hi))
(assert (! (= (+ x 1) 3) :named equation))
(check-sat-using (then lia2card smt))
(get-unsat-core)
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	got := strings.Join(strings.Fields(out.String()), " ")
	if !strings.HasPrefix(got, "unsat ") || !strings.Contains(got, "hi") || !strings.Contains(got, "equation") {
		t.Fatal("unsat core omitted required assumptions", got)
	}
}
