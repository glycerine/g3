package api

import "testing"

func TestAPITacticUsingParametersRejectsUnknownParameter(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	simplify, err := f.c.Tactic("simplify")
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, simplify.Close()) })
	parameters, err := f.c.NewParameterSet()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, parameters.Close()) })
	astTestMust(t, parameters.SetBool("not-a-simplifier-option", true))
	if _, err = simplify.UsingParameters(parameters); err == nil {
		t.Fatal("unknown tactic parameter accepted")
	}
}
