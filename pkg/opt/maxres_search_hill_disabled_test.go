package opt_test

import "testing"

func TestMaxResSearchWithoutHillClimb(t *testing.T) {
	script := "(set-option :opt.maxres.hill_climb false)\n(declare-const p Bool)\n(assert-soft p :weight 9)\n(assert-soft (not p) :weight 2)\n(check-sat)\n(get-objectives)\n"
	got := runMaxResScript(t, script)
	if got != "sat\n(objectives\n ( 2)\n)\n" {
		t.Fatalf("output %q", got)
	}
}
