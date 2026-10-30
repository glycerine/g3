package opt_test

import "testing"

func TestMaxResSearchChoosesCheaperViolation(t *testing.T) {
	script := "(declare-const p Bool)\n(assert-soft p :weight 2)\n(assert-soft (not p) :weight 1)\n(check-sat)\n(get-objectives)\n"
	got := runMaxResScript(t, script)
	if got != "sat\n(objectives\n ( 1)\n)\n" {
		t.Fatalf("output %q", got)
	}
}
