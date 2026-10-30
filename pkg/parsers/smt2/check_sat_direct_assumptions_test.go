package smt2

import "testing"

func TestSMT2CheckSatAcceptsDirectBooleanAssumptions(t *testing.T) {
	script := "(declare-const a Bool)(declare-const b Bool)" +
		"(assert (not a))(check-sat a)(check-sat b)(check-sat (not a))"
	output, err := runApplyTacticScript(t, script)
	if err != nil {
		t.Fatal(err)
	}
	if output != "unsat\nsat\nsat\n" {
		t.Fatal(output)
	}
}
