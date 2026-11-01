package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSMTLibRunnerRecursiveDatatypeQETactic(t *testing.T) {
	script := `(set-logic ALL)
(declare-datatypes ((List 0)) (((nil) (cons (head Int) (tail List)))))
(declare-const y List)
(assert (exists ((x List)) (and ((_ is cons) x) (= x y))))
(push)
(assert (= y nil))
(check-sat-using (then qe smt))
(pop)
(assert (= y (cons 1 nil)))
(check-sat-using (then qe smt))
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.Join(strings.Fields(out.String()), " "); got != "unsat sat" {
		t.Fatalf("datatype QE tactic result: %s", got)
	}
}

func TestSMTLibRunnerRecursiveDatatypeQEFreshVariable(t *testing.T) {
	script := `(set-logic ALL)
(declare-datatypes ((List 0)) (((nil) (cons (head Int) (tail List)))))
(declare-const y List)
(assert (exists ((x List)) (= x y)))
(push)
(assert (= y nil))
(check-sat-using (then qe smt))
(pop)
(assert (= y (cons 1 nil)))
(check-sat-using (then qe smt))
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.Join(strings.Fields(out.String()), " "); got != "sat sat" {
		t.Fatalf("datatype QE fresh-variable result: %s", got)
	}
}

func TestSMTLibRunnerDatatypeQEBothEqualityPolarities(t *testing.T) {
	script := `(set-logic ALL)
(declare-datatypes ((List 0)) (((nil) (cons (head Int) (tail List)))))
(declare-const y List)
(declare-const b Bool)
(assert (exists ((x List)) (and ((_ is cons) x) (or (and b (= x y)) (and (not b) (not (= x y)))))))
(assert (= y nil))
(push)
(assert b)
(check-sat-using (then qe smt))
(pop)
(assert (not b))
(check-sat-using (then qe smt))
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.Join(strings.Fields(out.String()), " "); got != "unsat sat" {
		t.Fatalf("datatype QE polarity result: %s", got)
	}
}

func TestSMTLibRunnerQEBoundedCooperResidue(t *testing.T) {
	script := `(set-logic LIA)
(declare-const y Int)
(assert (exists ((x Int)) (= (mod (+ (* 2 x) y) 7) 0)))
(assert (= y 3))
(check-sat-using (then qe smt))
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.Join(strings.Fields(out.String()), " "); got != "sat" {
		t.Fatalf("QE bounded-residue result: %s", got)
	}
}

func TestSMTLibRunnerQENestedModuloProjection(t *testing.T) {
	script := `(set-logic LIA)
(declare-const y Int)
(assert (exists ((x Int)) (= (mod (+ (* 2 x) y) 4) 1)))
(push)
(assert (= y 2))
(check-sat-using (then qe smt))
(pop)
(assert (= y 1))
(check-sat-using (then qe smt))
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.Join(strings.Fields(out.String()), " "); got != "unsat sat" {
		t.Fatalf("nested modulo QE result: %s", got)
	}
}

func TestSMTLibRunnerQEArrayEquations(t *testing.T) {
	script := `(set-logic ALL)
(declare-const B (Array Int Int))
(declare-const i Int)
(declare-const v Int)
(assert (exists ((A (Array Int Int))) (and (= A B) (= (select A i) v))))
(push)
(assert (not (= (select B i) v)))
(check-sat-using (then qe smt))
(pop)
(assert (= (select B i) v))
(check-sat-using (then qe smt))
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.Join(strings.Fields(out.String()), " "); got != "unsat sat" {
		t.Fatalf("array QE result: %s", got)
	}
}

func TestSMTLibRunnerQEArrayTwoSelects(t *testing.T) {
	script := `(set-logic ALL)
(declare-const i Int)
(declare-const j Int)
(declare-const v Int)
(declare-const w Int)
(assert (exists ((A (Array Int Int))) (and (= (select A i) v) (= (select A j) w))))
(assert (= i j))
(push)
(assert (not (= v w)))
(check-sat-using (then qe smt))
(pop)
(assert (= v w))
(check-sat-using (then qe smt))
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.Join(strings.Fields(out.String()), " "); got != "unsat sat" {
		t.Fatalf("two-select QE result: %s", got)
	}
}

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
