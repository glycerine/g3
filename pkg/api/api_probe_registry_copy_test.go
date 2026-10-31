package api

import "testing"

func TestAPINamedProbeRegistryReturnsCopies(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	first, err := f.c.ProbeDescriptors()
	astTestMust(t, err)
	first[0].Name = "changed"
	first[0].Description = "changed"
	second, err := f.c.ProbeDescriptors()
	astTestMust(t, err)
	if second[0].Name != "ackr-bound-probe" || second[0].Description != "A probe to give an upper bound of Ackermann congruence lemmas that a formula might generate." {
		t.Fatal("caller mutated registry", second[0])
	}
	names, err := f.c.ProbeNames()
	astTestMust(t, err)
	names[1] = "changed"
	again, err := f.c.ProbeNames()
	astTestMust(t, err)
	if again[1] != "is-unbounded" {
		t.Fatal("caller mutated names", again[1])
	}
}
