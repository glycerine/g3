package api

import "testing"

func TestAPITacticUsingParametersValidatesAndCopies(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	simplify, err := f.c.Tactic("simplify")
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, simplify.Close()) })
	parameters, err := f.c.NewParameterSet()
	astTestMust(t, err)
	astTestMust(t, parameters.SetBool("elim-and", true))
	configured, err := simplify.UsingParameters(parameters)
	astTestMust(t, err)
	astTestMust(t, parameters.SetBool("elim-and", false))
	astTestMust(t, parameters.Close())
	if _, err = configured.Help(); err != nil {
		t.Fatal("configured tactic metadata", err)
	}
	astTestMust(t, configured.Close())
}
