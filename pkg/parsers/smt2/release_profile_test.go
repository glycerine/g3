package smt2

import (
	"strings"
	"testing"
)

// AUDIT.md FUZZ-F7: g3 follows the _EXTERNAL_RELEASE build profile, which
// registers neither the polynomial/subpaving extra commands nor apply's
// print_proof parameter.
func TestSMT2ReleaseProfileHelpOmitsInternalCommands(t *testing.T) {
	for _, script := range []string{"(help)", "(help apply)"} {
		output, err := runApplyTacticScript(t, script)
		if err != nil {
			t.Fatal(script, err)
		}
		for _, line := range strings.Split(output, "\n") {
			if strings.HasPrefix(line, " (to-poly") || strings.HasPrefix(line, " (poly/") || strings.HasPrefix(line, " (to-subpaving") || strings.Contains(line, "print proof associated with each assertion") {
				t.Fatalf("%s lists an internal-profile entry: %q", script, line)
			}
		}
	}
	for _, name := range []string{"to-poly", "poly/factor", "poly/isolate-roots", "to-subpaving"} {
		if _, err := runApplyTacticScript(t, "(help "+name+")"); err == nil {
			t.Fatal("help knows", name)
		}
	}
}

func TestSMT2ReleaseProfileExtraCommandsUnsupported(t *testing.T) {
	output, err := runApplyTacticScript(t, "(declare-const x Real)\n(to-poly (* (+ x 1) (+ x 1)))\n(poly/factor (- (* x x) 1))\n(poly/isolate-roots (- (* x x) 2))\n(to-subpaving (+ (* x x) 1))\n")
	if err != nil {
		t.Fatal(err)
	}
	if output != "unsupported\nunsupported\nunsupported\nunsupported\n" {
		t.Fatalf("%q", output)
	}
}
