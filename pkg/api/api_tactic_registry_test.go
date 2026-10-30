package api

import "testing"

func TestAPITacticRegistryCountIndexAndNames(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	names, err := f.c.TacticNames()
	astTestMust(t, err)
	count, err := f.c.TacticCount()
	astTestMust(t, err)
	if len(names) != int(count) || len(names) < 70 {
		t.Fatal("registry size", len(names), count)
	}
	for _, index := range []uint32{0, 6, count - 3, count - 1} {
		name, nameErr := f.c.TacticName(index)
		astTestMust(t, nameErr)
		if name != names[index] {
			t.Fatal("indexed tactic", index, name, names[index])
		}
	}
	if yes, lookupErr := f.c.HasTactic("simplify"); lookupErr != nil || !yes {
		t.Fatal("known tactic lookup", yes, lookupErr)
	}
	if yes, lookupErr := f.c.HasTactic("not-a-retained-tactic"); lookupErr != nil || yes {
		t.Fatal("unknown tactic lookup", yes, lookupErr)
	}
}
