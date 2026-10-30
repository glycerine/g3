package api

import "testing"

func TestAPINamedProbeRegistrySourceOrder(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	want := []string{"memory", "depth", "size", "num-exprs", "num-consts", "num-bool-consts", "num-arith-consts", "num-bv-consts", "produce-proofs", "produce-model", "produce-unsat-cores", "has-quantifiers", "has-patterns", "is-propositional", "is-qfbv", "is-qfaufbv"}
	names, err := f.c.ProbeNames()
	astTestMust(t, err)
	count, err := f.c.ProbeCount()
	astTestMust(t, err)
	if int(count) != len(want) || len(names) != len(want) {
		t.Fatalf("registry size: %d %d, want %d", count, len(names), len(want))
	}
	for i := range want {
		descriptor, descriptorErr := f.c.ProbeDescriptor(uint32(i))
		astTestMust(t, descriptorErr)
		if names[i] != want[i] || descriptor.Name != want[i] || descriptor.Description == "" {
			t.Fatalf("registry entry %d: %#v %q, want %q", i, descriptor, names[i], want[i])
		}
	}
}
