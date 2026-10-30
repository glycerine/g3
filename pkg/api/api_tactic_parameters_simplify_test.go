package api

import (
	"strings"
	"testing"
)

func TestAPITacticParameterDescriptionsFromConcreteTactic(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	simplify, err := f.c.Tactic("simplify")
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, simplify.Close()) })
	descriptions, err := simplify.ParameterDescriptions()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, descriptions.Close()) })
	if size, sizeErr := descriptions.Size(); sizeErr != nil || size == 0 {
		t.Fatal("simplify parameters", size, sizeErr)
	}
	for _, name := range []string{"elim_and", "max_steps", "som", "sort_store"} {
		documentation, documentationErr := descriptions.Documentation(name)
		if documentationErr != nil || strings.TrimSpace(documentation) == "" {
			t.Fatal("parameter", name, documentation, documentationErr)
		}
	}
}
