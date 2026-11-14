package core

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

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
	idom := func(h ast.ASTHandle) ast.ASTHandle {
		v, ok, e := d.idom(h)
		domMust(t, e)
		if !ok {
			t.Fatal("missing idom")
		}
		return v
	}
	if idom(f.a) != root {
		t.Fatal("shared diamond idom", idom(f.a), root)
	}
	if idom(f.b) != left || idom(f.c) != right {
		t.Fatal("branch idoms")
	}
	domMust(t, d.reset())
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
	domMust(t, d.reset())
}

// compute_post_order pushes every unmarked argument, so the last argument
// is numbered first (AUDIT.md FUZZ_B-F8).
func TestExpressionDominatorsPostOrderLastArgumentFirst(t *testing.T) {
	f := newDomFixture(t)
	root, e := f.m.MkOr(f.a, f.b)
	domMust(t, e)
	d := newExpressionDominators(f.m)
	ok, e := d.compile(root)
	domMust(t, e)
	if !ok || len(d.post) != 3 || d.post[0] != f.b || d.post[1] != f.a || d.post[2] != root {
		t.Fatal(ok, d.post)
	}
	domMust(t, d.reset())
}
