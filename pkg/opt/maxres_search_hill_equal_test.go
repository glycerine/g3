package opt_test

import "testing"

func TestMaxResSearchHillClimbKeepsEqualWeightGroup(t *testing.T) {
	script := "(set-option :opt.maxres.hill_climb true)\n(declare-const p Bool)\n(assert-soft p :weight 4)\n(assert-soft (not p) :weight 4)\n(check-sat)\n(get-objectives)\n"
	got := runMaxResScript(t, script)
	if got != "sat\n(objectives\n ( 4)\n)\n" {
		t.Fatalf("output %q", got)
	}
}
