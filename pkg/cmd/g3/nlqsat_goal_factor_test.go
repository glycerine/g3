package main

import (
	"regexp"
	"testing"
	"time"
)

// AUDIT.md qe_03-F2: nlqsat's goal2nlsat reads factor (default true) from
// the constructor params, so the cubic below is factored into three atoms.
// The statistics are z3's for this script.
func TestNLQSATGoalCompilerFactors(t *testing.T) {
	script := `(declare-const x Real)
(assert (forall ((y Real)) (or (> (* (+ y 1.0) (+ y 2.0) (+ y 3.0)) 0.0) (< (* x y) 3.0) (= (* y y) (* 4.0 x)))))
(check-sat-using (then simplify nlqsat))
(get-info :all-statistics)`
	got := runScriptWithin(t, script, time.Minute)
	for _, want := range []string{`(?m)^sat$`, `:nlsat-conflicts +11\b`, `:nlsat-decisions +6\b`} {
		if !regexp.MustCompile(want).MatchString(got) {
			t.Fatalf("output lacks %s:\n%s", want, got)
		}
	}
}
