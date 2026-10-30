package opt_test

import "testing"

func TestMaxResPrimalDualSearchRespectsHardFormula(t *testing.T) {
	script := "(set-option :opt.maxsat_engine pd-maxres)\n(declare-const p Bool)\n(assert (not p))\n(assert-soft p :weight 6)\n(check-sat)\n(get-objectives)\n"
	got := runMaxResScript(t, script)
	if got != "sat\n(objectives\n ( 6)\n)\n" {
		t.Fatalf("output %q", got)
	}
}
