package opt_test

import "testing"

func TestMaxResSearchHillClimbWeightGroups(t *testing.T) {
	script := "(set-option :opt.maxres.hill_climb true)\n(declare-const p Bool)\n(declare-const q Bool)\n(assert (not (and p q)))\n(assert-soft p :weight 7)\n(assert-soft q :weight 3)\n(check-sat)\n(get-objectives)\n"
	got := runMaxResScript(t, script)
	if got != "sat\n(objectives\n ( 3)\n)\n" {
		t.Fatalf("output %q", got)
	}
}
