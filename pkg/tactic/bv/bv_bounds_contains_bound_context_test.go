package bv

import "testing"

func TestBVBoundsContainsBoundFromContext(t *testing.T) {
	f := newBVBoundsFixture(t)
	lower := f.ule(t, f.num(t, 4), f.x)
	ok, err := f.simp.AssertExpression(lower, false)
	sharingMust(t, err)
	if !ok {
		t.Fatal("context bound rejected")
	}
	upper := f.ule(t, f.x, f.num(t, 200))
	found, err := f.simp.containsBound(upper)
	sharingMust(t, err)
	if !found {
		t.Fatal("context variable not found")
	}
}
