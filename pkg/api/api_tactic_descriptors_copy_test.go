package api

import "testing"

func TestAPITacticDescriptorsAreCallerOwned(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	first, err := f.c.TacticDescriptors()
	astTestMust(t, err)
	first[0] = APITacticDescriptor{Name: "changed", Description: "changed"}
	second, err := f.c.TacticDescriptors()
	astTestMust(t, err)
	if second[0].Name != "qe-light" || second[0].Description != "apply light-weight quantifier elimination." {
		t.Fatal("caller changed descriptions", second[0])
	}
}
