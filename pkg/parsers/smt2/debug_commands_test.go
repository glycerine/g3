package smt2

import "testing"

func TestDebugExpressionCommands(t *testing.T) {
	script := `(declare-const x Int)
(dbg-set saved (+ x 1))
(dbg-pp-var saved)
(dbg-set saved (+ x 2))
(dbg-pp-var saved)
(dbg-size (+ x x))
(dbg-get-qbody body (forall ((y Int)) (> y 0)))
(dbg-pp-var body)`
	out, err := runApplyTacticScript(t, script)
	if err != nil {
		t.Fatal(err, out)
	}
	if want := "(+ x 1)\n(+ x 2)\n2\n(> (:var 0) 0)\n"; out != want {
		t.Fatalf("debug output %q, want %q", out, want)
	}
}

func TestDebugExpressionResetAndErrors(t *testing.T) {
	for _, script := range []string{
		`(dbg-pp-var absent)`,
		`(dbg-get-qbody saved true)`,
		`(dbg-set saved true)(reset)(dbg-pp-var saved)`,
	} {
		out, err := runApplyTacticScript(t, script)
		if err == nil {
			t.Fatalf("accepted invalid debug command %q: %s", script, out)
		}
	}
}

func TestDebugShiftAndSharedExpressions(t *testing.T) {
	script := `(declare-const x Int)
(dbg-get-qbody body (forall ((y Int)) (> y 0)))
(dbg-shift-vars body 2)
(dbg-pp-var body)
(dbg-num-shared (+ (* x x) (* x x)))
(dbg-num-shared (+ x x))`
	out, err := runApplyTacticScript(t, script)
	if err != nil {
		t.Fatal(err, out)
	}
	if want := "(> (:var 2) 0)\n1\n0\n"; out != want {
		t.Fatalf("debug output %q, want %q", out, want)
	}
}

func TestDebugSubstitutionBindingOrderAndReplacement(t *testing.T) {
	script := `(dbg-get-qbody s (forall ((x Int) (y Int)) (> x y)))
(dbg-set a 3)
(dbg-set b 5)
(dbg-subst s (a b) s)
(dbg-pp-var s)`
	out, err := runApplyTacticScript(t, script)
	if err != nil {
		t.Fatal(err, out)
	}
	if want := "(> 5 3)\n"; out != want {
		t.Fatalf("debug substitution %q, want %q", out, want)
	}
}

func TestDebugInstantiateQuantifiers(t *testing.T) {
	script := `(dbg-instantiate (forall ((x Int) (y Int)) (> x y)) (3 5))
(dbg-instantiate-nested (forall ((x Int)) (exists ((y Int)) (> x y))) (3))`
	out, err := runApplyTacticScript(t, script)
	if err != nil {
		t.Fatal(err, out)
	}
	if want := "(> 3 5)\n(> (:var 0) 3)\n"; out != want {
		t.Fatalf("debug instantiation %q, want %q", out, want)
	}
	for _, input := range []string{
		`(dbg-instantiate true ())`,
		`(dbg-instantiate (forall ((x Int)) (> x 0)) ())`,
		`(dbg-instantiate (forall ((x Int)) (> x 0)) (true))`,
		`(dbg-instantiate-nested (forall ((x Int)) (> x 0)) (1))`,
	} {
		if out, err := runApplyTacticScript(t, input); err == nil {
			t.Fatalf("accepted invalid debug instantiation %q: %s", input, out)
		}
	}
}

func TestDebugEliminateUnusedVariables(t *testing.T) {
	script := `(dbg-elim-unused-vars (forall ((x Int) (y Int)) (> x 0)))
(dbg-elim-unused-vars true)
(echo "after")`
	out, err := runApplyTacticScript(t, script)
	if err != nil {
		t.Fatal(err, out)
	}
	if want := "(forall ((x Int)) (> x 0))\ntrueafter\n"; out != want {
		t.Fatalf("debug elimination %q, want %q", out, want)
	}
}
