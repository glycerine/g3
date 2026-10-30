package bv

import "testing"

func TestBVBoundsContainsBoundRepeatedVariable(t *testing.T) {
	f := newBVBoundsFixture(t)
	lower := f.ule(t, f.num(t, 4), f.x)
	upper := f.ule(t, f.x, f.num(t, 200))
	root, err := f.m.MkAnd(lower, upper)
	sharingMust(t, err)
	f.pin(t, root)
	found, err := f.simp.containsBound(root)
	sharingMust(t, err)
	if !found {
		t.Fatal("repeated bound variable not found")
	}
}
