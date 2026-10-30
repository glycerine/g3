package opt_test

import "testing"

func TestMaxResSearchRespectsHardConstraint(t *testing.T) {
	script := "(declare-const p Bool)\n(assert (not p))\n(assert-soft p :weight 3)\n(check-sat)\n(get-objectives)\n"
	got := runMaxResScript(t, script)
	if got != "sat\n(objectives\n ( 3)\n)\n" {
		t.Fatalf("output %q", got)
	}
}
