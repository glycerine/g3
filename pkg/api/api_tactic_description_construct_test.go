package api

import "testing"

func TestAPITacticDescriptionMatchesConstructibleFactory(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	for _, name := range []string{"skip", "fail", "simplify", "sat"} {
		description, err := f.c.TacticDescription(name)
		astTestMust(t, err)
		if description == "" {
			t.Fatal("empty description", name)
		}
		created, err := f.c.Tactic(name)
		astTestMust(t, err)
		astTestMust(t, created.Close())
	}
}
