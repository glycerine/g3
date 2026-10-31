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

func TestSMTLibRunnerArithmeticInequalityQETactic(t *testing.T) {
	script := `(set-logic LIA)
(push)
(assert (exists ((x Int)) (> x 0)))
(check-sat-using (then qe smt))
(pop)
(assert (forall ((x Int)) (>= x 0)))
(check-sat-using (then qe smt))
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.Join(strings.Fields(out.String()), " "); got != "sat unsat" {
		t.Fatalf("arithmetic inequality QE tactic result: %s", got)
	}
}

func TestSMTLibRunnerQEInequalityIsNotEquality(t *testing.T) {
	script := `(set-logic LIA)
(assert (exists ((x Int)) (and (<= x 2) (< x 2))))
(check-sat-using (then qe smt))
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.TrimSpace(out.String()); got != "sat" {
		t.Fatalf("inequality QE tactic result: %s", got)
	}
}

func TestSMTLibRunnerQEOpposingBounds(t *testing.T) {
	script := `(set-logic LIA)
(declare-const y Int)
(assert (not (exists ((x Int)) (and (>= x y) (<= x y)))))
(check-sat-using (then qe smt))
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.TrimSpace(out.String()); got != "unsat" {
		t.Fatalf("opposing bounds QE tactic result: %s", got)
	}
}

func TestSMTLibRunnerQESymbolicIntegerBounds(t *testing.T) {
	script := `(set-logic LIA)
(declare-const y Int)
(assert (exists ((x Int)) (and (>= x y) (<= x 0))))
(push)
(assert (> y 0))
(check-sat-using (then qe smt))
(pop)
(assert (= y (- 1)))
(check-sat-using (then qe smt))
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.Join(strings.Fields(out.String()), " "); got != "unsat sat" {
		t.Fatalf("symbolic integer QE result: %s", got)
	}
}

func TestSMTLibRunnerQESymbolicRealBounds(t *testing.T) {
	script := `(set-logic LRA)
(declare-const y Real)
(assert (exists ((x Real)) (and (> x 0.0) (< x y))))
(push)
(assert (<= y 0.0))
(check-sat-using (then qe smt))
(pop)
(assert (= y 1.0))
(check-sat-using (then qe smt))
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.Join(strings.Fields(out.String()), " "); got != "unsat sat" {
		t.Fatalf("symbolic real QE result: %s", got)
	}
}

func TestSMTLibRunnerQEMixedCoefficientBounds(t *testing.T) {
	script := `(set-logic LIA)
(declare-const y Int)
(declare-const z Int)
(assert (exists ((x Int)) (and (<= (* 2 x) y) (>= x z))))
(push)
(assert (= y 1))
(assert (= z 1))
(check-sat-using (then qe smt))
(pop)
(assert (= y 3))
(assert (= z 1))
(check-sat-using (then qe smt))
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.Join(strings.Fields(out.String()), " "); got != "unsat sat" {
		t.Fatalf("mixed coefficient QE result: %s", got)
	}
}

func TestSMTLibRunnerQESmallDivisibility(t *testing.T) {
	script := `(set-logic LIA)
(declare-const y Int)
(assert (exists ((x Int)) (= (* 2 x) y)))
(push)
(assert (= y 1))
(check-sat-using (then qe smt))
(pop)
(assert (= y 2))
(check-sat-using (then qe smt))
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.Join(strings.Fields(out.String()), " "); got != "unsat sat" {
		t.Fatalf("small divisibility QE result: %s", got)
	}
}

func TestSMTLibRunnerQEBitVectorQuantifier(t *testing.T) {
	script := `(set-logic BV)
(declare-const y (_ BitVec 2))
(push)
(assert (exists ((x (_ BitVec 2))) (= (bvadd x y) (_ bv1 2))))
(check-sat-using (then qe smt))
(pop)
(assert (forall ((x (_ BitVec 2))) (= (bvadd x y) (_ bv1 2))))
(check-sat-using (then qe smt))
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.Join(strings.Fields(out.String()), " "); got != "sat unsat" {
		t.Fatalf("bit-vector QE tactic result: %s", got)
	}
}

func TestSMTLibRunnerQESymbolicDivisibility(t *testing.T) {
	script := `(set-logic LIA)
(declare-const y Int)
(assert (exists ((x Int)) (= (* 4 x) y)))
(push)
(assert (= y 2))
(check-sat-using (then qe smt))
(pop)
(assert (= y 4))
(check-sat-using (then qe smt))
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.Join(strings.Fields(out.String()), " "); got != "unsat sat" {
		t.Fatalf("symbolic divisibility QE result: %s", got)
	}
}
