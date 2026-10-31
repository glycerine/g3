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

func TestSMTLibRunnerArithmeticEquationQETactic(t *testing.T) {
	script := `(set-logic LIA)
(set-option :produce-models true)
(declare-const y Int)
(assert (exists ((x Int)) (and (= x y) (> x 0))))
(assert (= y 2))
(check-sat-using (then qe smt))
(get-value (y))
(push)
(assert (<= y 0))
(check-sat-using (then qe smt))
(pop)
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.Join(strings.Fields(out.String()), " "); got != "sat ((y 2)) unsat" {
		t.Fatalf("arithmetic QE tactic result: %s", got)
	}
}

func TestSMTLibRunnerMixedQETactic(t *testing.T) {
	script := `(set-logic ALL)
(declare-const y Int)
(assert (exists ((b Bool) (x Int)) (and b (= x y) (> x 0))))
(assert (= y 3))
(check-sat-using (then qe smt))
(push)
(assert (<= y 0))
(check-sat-using (then qe smt))
(pop)
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.Join(strings.Fields(out.String()), " "); got != "sat unsat" {
		t.Fatalf("mixed QE tactic result: %s", got)
	}
}

func TestSMTLibRunnerFreshIntegerEquationQETactic(t *testing.T) {
	script := `(set-logic LIA)
(assert (not (exists ((x Int) (y Int)) (= (+ (* 2 x) (* 3 y) 1) 0))))
(check-sat-using (then qe smt))
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.TrimSpace(out.String()); got != "unsat" {
		t.Fatalf("fresh integer QE tactic result: %s", got)
	}
}
