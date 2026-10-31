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
