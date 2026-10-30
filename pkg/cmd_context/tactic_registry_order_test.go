package cmd_context

import "testing"

func TestBuiltinTacticRegistryRetainsFactoryOrder(t *testing.T) {
	names := BuiltinTacticNames()
	if len(names) != int(BuiltinTacticCount()) || len(names) < 70 {
		t.Fatal("registry size", len(names), BuiltinTacticCount())
	}
	wantPrefix := []string{"qe-light", "qsat", "nlqsat", "nlqe", "qe2", "qe_rec", "smt", "nlsat"}
	for i := range wantPrefix {
		if names[i] != wantPrefix[i] {
			t.Fatalf("prefix %d: %q, want %q", i, names[i], wantPrefix[i])
		}
	}
	wantSuffix := []string{"auflia", "auflira", "aufnira", "skip", "fail", "fail-if-undecided"}
	for i := range wantSuffix {
		index := len(names) - len(wantSuffix) + i
		if names[index] != wantSuffix[i] {
			t.Fatalf("suffix %d: %q, want %q", index, names[index], wantSuffix[i])
		}
	}
}
