package api

import "testing"

func TestAPITacticDescriptorsFollowRegistryOrder(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	names, err := f.c.TacticNames()
	astTestMust(t, err)
	descriptors, err := f.c.TacticDescriptors()
	astTestMust(t, err)
	if len(descriptors) != len(names) {
		t.Fatal("descriptor count", len(descriptors), len(names))
	}
	for i := range names {
		if descriptors[i].Name != names[i] || descriptors[i].Description == "" {
			t.Fatal("descriptor", i, descriptors[i], names[i])
		}
	}
}
