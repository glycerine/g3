package api

import "testing"

func TestASTVectorOwnsElementsUntilClose(t *testing.T) {
	f, a, _ := astVectorFixture(t)
	base, err := f.m.GetRefCount(a)
	astTestMust(t, err)
	v, err := f.c.NewASTVectorValues(a, a)
	astTestMust(t, err)
	retained, err := f.m.GetRefCount(a)
	astTestMust(t, err)
	if retained != base+2 {
		t.Fatalf("vector references: got %d want %d", retained, base+2)
	}
	astTestMust(t, v.Close())
	after, err := f.m.GetRefCount(a)
	astTestMust(t, err)
	if after != base {
		t.Fatalf("references after close: got %d want %d", after, base)
	}
}
