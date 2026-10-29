package api

import "testing"

func TestSMTContextExplicitASTReferencesBalance(t *testing.T) {
	f := newSMTAPIFixture(t, "ALL", nil)
	value := f.pin(f.c.Bool(true))
	before, err := f.m.GetRefCount(value)
	astTestMust(t, err)
	astTestMust(t, f.c.IncASTRef(value))
	after, err := f.m.GetRefCount(value)
	astTestMust(t, err)
	if after != before+1 {
		t.Fatal("incremented reference count", before, after)
	}
	astTestMust(t, f.c.DecASTRef(value))
	after, err = f.m.GetRefCount(value)
	astTestMust(t, err)
	if after != before {
		t.Fatal("balanced reference count", before, after)
	}
}
