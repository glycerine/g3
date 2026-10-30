package api

import (
	"strings"
	"testing"
)

func TestAPITacticHelpUsesCollectedParameterDescriptions(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	simplify, err := f.c.Tactic("simplify")
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, simplify.Close()) })
	help, err := simplify.Help()
	astTestMust(t, err)
	for _, text := range []string{"elim_and", "max_steps", "sort_store"} {
		if !strings.Contains(help, text) {
			t.Fatal("help missing", text, help)
		}
	}
	skip, err := f.c.SkipTactic()
	astTestMust(t, err)
	if help, err = skip.Help(); err != nil || help != "" {
		t.Fatal("empty primitive help", help, err)
	}
	astTestMust(t, skip.Close())
}
