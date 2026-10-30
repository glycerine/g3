package opt_test

import "testing"

func TestMaxResPrimalDualSearchWeightedConflict(t *testing.T) {
	script := "(set-option :opt.maxsat_engine pd-maxres)\n(declare-const p Bool)\n(assert-soft p :weight 4)\n(assert-soft (not p) :weight 1)\n(check-sat)\n(get-objectives)\n"
	got := runMaxResScript(t, script)
	if got != "sat\n(objectives\n ( 1)\n)\n" {
		t.Fatalf("output %q", got)
	}
}
