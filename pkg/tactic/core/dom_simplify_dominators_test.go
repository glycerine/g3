package core

import "testing"

func TestExpressionDominatorsDiamondAndTree(t *testing.T) {
	f := newDomFixture(t)
	left, e := f.m.MkAnd(f.a, f.b)
	domMust(t, e)
	right, e := f.m.MkOr(f.a, f.c)
	domMust(t, e)
	root, e := f.m.MkAnd(left, right)
	domMust(t, e)
	d := newExpressionDominators(f.m)
	ok, e := d.compile(root)
	domMust(t, e)
	if !ok {
		t.Fatal("compile")
	}
	if d.idom[f.a] != root {
		t.Fatal("shared diamond idom", d.idom[f.a], root)
	}
	if d.idom[f.b] != left || d.idom[f.c] != right {
		t.Fatal("branch idoms", d.idom)
	}
}
func TestExpressionDominatorsPostOrderEndsAtRoot(t *testing.T) {
	f := newDomFixture(t)
	root, e := f.m.MkOr(f.a, f.b)
	domMust(t, e)
	d := newExpressionDominators(f.m)
	ok, e := d.compile(root)
	domMust(t, e)
	if !ok || len(d.post) == 0 || d.post[len(d.post)-1] != root {
		t.Fatal(ok, d.post)
	}
}
