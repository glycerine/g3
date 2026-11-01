package cmd_context

import "testing"

func TestBuiltinTacticDescriptionsRetainSourceText(t *testing.T) {
	want := map[string]string{
		"skip":                    "do nothing tactic.",
		"sat":                     "(try to) solve goal using a SAT solver.",
		"qe-light":                "apply light-weight quantifier elimination.",
		"qe-sat":                  "check satisfiability of quantified formulas using quantifier elimination.",
		"ackermannize_bv":         "A tactic for performing full Ackermannization on bv instances.",
		"dom-simplify":            "apply dominator simplification rules.",
		"ctx-solver-simplify":     "apply solver-based contextual simplification rules.",
		"tseitin-cnf-core":        "convert goal into CNF using tseitin-like encoding (note: quantifiers are ignored). This tactic does not apply required simplifications to the input goal like the tseitin-cnf tactic.",
		"propagate-bv-bounds":     "propagate bit-vector bounds by simplifying implied or contradictory bounds.",
		"propagate-bv-bounds-new": "propagate bit-vector bounds by simplifying implied or contradictory bounds.",
	}
	for name, expected := range want {
		got, err := BuiltinTacticDescription(name)
		contextMust(t, err)
		if got != expected {
			t.Fatalf("%s: %q, want %q", name, got, expected)
		}
	}
}
