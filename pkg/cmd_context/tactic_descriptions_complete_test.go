package cmd_context

import "testing"

func TestBuiltinTacticDescriptionsCoverRegistry(t *testing.T) {
	names := BuiltinTacticNames()
	if len(builtinTacticDescriptions) != len(names) {
		t.Fatalf("description coverage: %d, want %d", len(builtinTacticDescriptions), len(names))
	}
	for _, name := range names {
		description, err := BuiltinTacticDescription(name)
		contextMust(t, err)
		if description == "" {
			t.Fatal("empty description", name)
		}
	}
}
