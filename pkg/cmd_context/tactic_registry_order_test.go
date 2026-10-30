package cmd_context

import "testing"

func TestBuiltinTacticRegistryRetainsFactoryOrder(t *testing.T) {
	names := BuiltinTacticNames()
	if len(names) != int(BuiltinTacticCount()) || len(names) < 70 {
		t.Fatal("registry size", len(names), BuiltinTacticCount())
	}
	wantBV := []string{"bv_bound_chk", "propagate-bv-bounds", "propagate-bv-bounds-new", "reduce-bv-size"}
	position := make(map[string]int, len(names))
	for i, name := range names {
		position[name] = i
	}
	for i, name := range wantBV {
		if !IsBuiltinTactic(name) {
			t.Fatalf("missing BV bounds registration %q", name)
		}
		if i > 0 && position[name] != position[wantBV[i-1]]+1 {
			t.Fatalf("BV bounds registration order: %v", wantBV)
		}
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
