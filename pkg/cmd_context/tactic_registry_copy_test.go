package cmd_context

import "testing"

func TestBuiltinTacticRegistryReturnsIndependentSnapshots(t *testing.T) {
	first := BuiltinTacticNames()
	first[0] = "changed"
	second := BuiltinTacticNames()
	if second[0] != "ackermannize_bv" {
		t.Fatal("caller changed registry", second[0])
	}
	last, err := BuiltinTacticName(BuiltinTacticCount() - 1)
	contextMust(t, err)
	if last != "ufbv" {
		t.Fatal("last tactic", last)
	}
}
