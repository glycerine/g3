package bv

import "testing"

func TestBVBoundsContainsExpressionNestedTarget(t *testing.T) {
	f := newBVBoundsFixture(t)
	bound := f.ule(t, f.x, f.num(t, 12))
	nested, err := f.m.MkAnd(bound, f.ule(t, f.num(t, 2), f.x))
	sharingMust(t, err)
	f.pin(t, nested)
	contains, err := f.simp.containsExpression(nested, f.x)
	sharingMust(t, err)
	if !contains {
		t.Fatal("nested target not found")
	}
	missing, err := f.simp.containsExpression(nested, f.num(t, 99))
	sharingMust(t, err)
	if missing {
		t.Fatal("unrelated numeral reported present")
	}
}
