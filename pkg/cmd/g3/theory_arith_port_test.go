package main

import (
	"testing"
	"time"
)

// PERF.md B5: with the theory_arith port, fixed_var_eh runs when f(1), f(2)
// and f(3) become fixed during bound assertion and equates f(3) with 6, so
// the goal is refuted at once, as in C++. The replay theory never terminates.
func TestTheoryArithPortSolvesFixedVarEquality(t *testing.T) {
	t.Setenv("G3_THEORY_ARITH", "port")
	const script = `(set-logic AUFLIA)
(declare-fun f (Int) Int)
(assert (forall ((x Int)) (! (= (f (+ x 1)) (+ (f x) 2)) :pattern ((f x)))))
(assert (= (f 0) 0))
(assert (not (= (f 3) 6)))
(check-sat)
`
	if got := runScriptWithin(t, script, 20*time.Second); got != "unsat\n" {
		t.Fatalf("got %q, want unsat", got)
	}
}

// Ground instances of B5: the equality adapter must create the triangle
// axioms itself (arith_eq_adapter::mk_axioms), or the port answers sat.
func TestTheoryArithPortGroundEqualityAdapter(t *testing.T) {
	t.Setenv("G3_THEORY_ARITH", "port")
	const script = `(set-logic QF_UFLIA)
(declare-fun f (Int) Int)
(assert (= (f (+ 0 1)) (+ (f 0) 2)))
(assert (= (f (+ 1 1)) (+ (f 1) 2)))
(assert (= (f (+ 2 1)) (+ (f 2) 2)))
(assert (= (f 0) 0))
(assert (not (= (f 3) 6)))
(check-sat)
`
	if got := runScriptWithin(t, script, 20*time.Second); got != "unsat\n" {
		t.Fatalf("got %q, want unsat", got)
	}
}

// ctx.relevancy() is m_relevancy_lvl > 0. At relevancy level 0 (QF_LIA) the
// mod/div axioms are created at internalization; reading the presence of
// the relevancy propagator instead dropped them and answered sat.
func TestTheoryArithPortDivisionAxiomsWithoutRelevancy(t *testing.T) {
	t.Setenv("G3_THEORY_ARITH", "port")
	const script = `(set-logic QF_LIA)
(declare-const x Int)
(assert (= (mod x 3) 2))
(push 1)
(assert (= (div x 3) 0))
(assert (> x 3))
(check-sat)
(pop 1)
(assert (= (div x 3) 1))
(check-sat)
(get-value (x))
`
	if got := runScriptWithin(t, script, 20*time.Second); got != "unsat\nsat\n((x 5))\n" {
		t.Fatalf("got %q", got)
	}
}

// optsmt::add rewrites the objective, so minimize hands the theory
// (* -1 x), which internalize_term_core supports, not (- x).
func TestTheoryArithPortMinimizeObjective(t *testing.T) {
	t.Setenv("G3_THEORY_ARITH", "port")
	const script = `(set-option :produce-models true)
(set-logic QF_LIA)
(declare-const x Int)
(assert (and (>= x 0) (<= x 3)))
(push)
(maximize x)
(check-sat)
(get-objectives)
(pop)
(push)
(minimize x)
(check-sat)
(get-objectives)
(pop)
`
	want := "sat\n(objectives\n (x 3)\n)\nsat\n(objectives\n (x 0)\n)\n"
	if got := runScriptWithin(t, script, 20*time.Second); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
