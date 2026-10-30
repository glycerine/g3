package cmd_context

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
)

func TestBuiltinTacticRegistryMembershipAndBounds(t *testing.T) {
	seen := make(map[string]bool)
	for index, name := range BuiltinTacticNames() {
		if seen[name] || !IsBuiltinTactic(name) {
			t.Fatal("invalid registry entry", index, name)
		}
		seen[name] = true
		indexed, err := BuiltinTacticName(uint32(index))
		contextMust(t, err)
		if indexed != name {
			t.Fatal("indexed name", index, indexed, name)
		}
	}
	if IsBuiltinTactic("") || IsBuiltinTactic("not-a-retained-tactic") {
		t.Fatal("unknown tactic registered")
	}
	if _, err := BuiltinTacticName(BuiltinTacticCount()); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("registry bound", err)
	}
}
