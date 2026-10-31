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

func TestSMTLibRunnerEq2BVUnconvertedBounds(t *testing.T) {
	script := `(set-logic QF_LIA)
(declare-const x Int)
(declare-const y Int)
(assert (= x 1))
(assert (>= y 1))
(assert (<= y 0))
(check-sat-using (then eq2bv smt))
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.TrimSpace(out.String()); got != "unsat" {
		t.Fatalf("contradictory unconverted bounds: %s", got)
	}
}

func TestSMTLibRunnerEq2BVLowerBoundDoesNotWrap(t *testing.T) {
	script := `(set-logic QF_LIA)
(declare-const x Int)
(assert (= x 1))
(assert (>= x 5))
(assert (<= x 1))
(check-sat-using (then eq2bv smt))
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.TrimSpace(out.String()); got != "unsat" {
		t.Fatalf("wrapped lower bound: %s", got)
	}
}

func TestSMTLibRunnerEq2BVLowerBoundKeepsOtherValues(t *testing.T) {
	script := `(set-logic QF_LIA)
(declare-const x Int)
(assert (not (= x 1)))
(assert (>= x 5))
(check-sat-using (then eq2bv smt))
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.TrimSpace(out.String()); got != "sat" {
		t.Fatalf("lower bound lost a valid integer witness: %s", got)
	}
}

func TestSMTLibRunnerEq2BVUnboundedUint32Maximum(t *testing.T) {
	script := `(set-logic QF_LIA)
(declare-const x Int)
(assert (not (= x 4294967295)))
(assert (>= x 4294967295))
(check-sat-using (then eq2bv smt))
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.TrimSpace(out.String()); got != "sat" {
		t.Fatalf("lost witness above uint32 maximum: %s", got)
	}
}

func TestSMTLibRunnerEq2BVNegativeWitness(t *testing.T) {
	script := `(set-logic QF_LIA)
(set-option :produce-models true)
(declare-const x Int)
(assert (not (= x 0)))
(assert (<= x 0))
(check-sat-using (then eq2bv qfbv))
(get-value (x))
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.Join(strings.Fields(out.String()), " "); got != "sat ((x (- 1)))" {
		t.Fatalf("lost negative witness or reconstructed model: %s", got)
	}
}

func TestSMTLibRunnerEq2BVNegativeSentinelDoesNotCollide(t *testing.T) {
	script := `(set-logic QF_LIA)
(set-option :produce-models true)
(declare-const x Int)
(assert (<= x 3))
(assert (not (= x 0)))
(assert (not (= x 1)))
(assert (not (= x 2)))
(assert (not (= x 3)))
(check-sat-using (then eq2bv qfbv))
(get-value (x))
`
	var out bytes.Buffer
	if err := run(strings.NewReader(script), &out); err != nil {
		t.Fatal(err, out.String())
	}
	if got := strings.Join(strings.Fields(out.String()), " "); got != "sat ((x (- 1)))" {
		t.Fatalf("negative sentinel collided with a numeral: %s", got)
	}
}
