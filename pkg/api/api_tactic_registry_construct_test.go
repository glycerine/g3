package api

import "testing"

func TestAPITacticRegistryNamesConstructConcreteTactics(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	for _, name := range []string{"skip", "fail", "simplify", "sat"} {
		created, err := f.c.Tactic(name)
		astTestMust(t, err)
		astTestMust(t, created.Close())
		astTestMust(t, created.Close())
	}
}
