package opt_test

import "testing"

func TestMaxResSearchAllSoftSatisfiable(t *testing.T) {
	got := runMaxResScript(t, "(declare-const p Bool)\n(assert-soft p :weight 7)\n(check-sat)\n(get-objectives)\n")
	if got != "sat\n(objectives\n ( 0)\n)\n" {
		t.Fatalf("output %q", got)
	}
}
