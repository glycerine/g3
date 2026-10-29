package core

import "testing"

func TestDomSimplifyCleanupAndTranslation(t *testing.T) {
	f := newDomFixture(t)
	r, e := NewDomSimplifyTactic(f.m, nil)
	domMust(t, e)
	t.Cleanup(func() { domMust(t, r.Close()) })
	root, e := f.m.MkAnd(f.a, f.b)
	domMust(t, e)
	ok, e := r.dominators.compile(root)
	domMust(t, e)
	if !ok || len(r.dominators.post) == 0 {
		t.Fatal("compile")
	}
	domMust(t, r.Cleanup())
	if len(r.dominators.post) != 0 {
		t.Fatal("cleanup")
	}
	copyT, e := r.Translate(f.m)
	domMust(t, e)
	copy := copyT.(*DomSimplifyTactic)
	t.Cleanup(func() { domMust(t, copy.Close()) })
	if copy.maxDepth != 1024 || len(copy.dominators.post) != 0 {
		t.Fatal("translation")
	}
}
