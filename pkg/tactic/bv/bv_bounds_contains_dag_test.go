package bv

import "testing"

func TestBVBoundsContainsExpressionSharedDAG(t *testing.T) {
	f := newBVBoundsFixture(t)
	shared := f.ule(t, f.num(t, 7), f.x)
	root, err := f.m.MkAnd(shared, shared)
	sharingMust(t, err)
	f.pin(t, root)
	found, err := f.simp.containsExpression(root, f.x)
	sharingMust(t, err)
	if !found {
		t.Fatal("shared child target not found")
	}
}
