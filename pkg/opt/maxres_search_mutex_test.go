package opt_test

import "testing"

func TestMaxResSearchWeightedMutex(t *testing.T) {
	script := "(declare-const p Bool)\n(declare-const q Bool)\n(assert (not (and p q)))\n(assert-soft p :weight 2)\n(assert-soft q :weight 3)\n(check-sat)\n(get-objectives)\n"
	got := runMaxResScript(t, script)
	if got != "sat\n(objectives\n ( 2)\n)\n" {
		t.Fatalf("output %q", got)
	}
}
