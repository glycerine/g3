package smt2

import "testing"

func TestSMT2CheckSatAssumingRetainsParenthesizedGrammar(t *testing.T) {
	script := "(declare-const a Bool)(assert (not a))" +
		"(check-sat-assuming (a))(check-sat-assuming ((not a)))"
	output, err := runApplyTacticScript(t, script)
	if err != nil {
		t.Fatal(err)
	}
	if output != "unsat\nsat\n" {
		t.Fatal(output)
	}
}
