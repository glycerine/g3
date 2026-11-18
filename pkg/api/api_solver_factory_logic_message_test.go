package api

import "testing"

// AUDIT.md cmd_context_02-F13: Z3_mk_solver_for_logic's message quotes
// the logic with single quotes.
func TestNewSolverForLogicUnknownLogicText(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	if _, err := f.c.NewSolverForLogic("FOO"); err == nil || err.Error() != "logic 'FOO' is not recognized" {
		t.Fatalf("error %v", err)
	}
}
