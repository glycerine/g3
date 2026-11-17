package ast

import "testing"

// The held reference operations keep the same reference counts as the
// checked ones, and an assignment or reset that releases the last
// reference still deletes the node.
func TestASTRefHeldMatchesChecked(t *testing.T) {
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	x, e := m.MkConst(astTestSymbol(t, m, "held-x"), b)
	astTestMust(t, e)
	astTestPin(t, m, x)
	y, e := m.MkConst(astTestSymbol(t, m, "held-y"), b)
	astTestMust(t, e)
	astTestPin(t, m, y)
	rc := func(h ASTHandle) uint32 {
		n, e := m.GetRefCount(h)
		astTestMust(t, e)
		return n
	}
	ref, e := m.NewRef(ASTHandle{})
	astTestMust(t, e)
	x0, y0 := rc(x), rc(y)
	astTestMust(t, m.SetRefHeld(ref, x))
	if got, e := m.RefGetHeld(ref); e != nil || got != x || rc(x) != x0+1 {
		t.Fatal("set held", got, e, rc(x))
	}
	astTestMust(t, m.SetRefHeld(ref, x))
	astTestMust(t, m.SetRefHeld(ref, y))
	if rc(x) != x0 || rc(y) != y0+1 {
		t.Fatal("reassign held", rc(x), rc(y))
	}
	astTestMust(t, m.ResetRefHeld(ref))
	if got, e := m.RefGetHeld(ref); e != nil || !got.IsNull() || rc(y) != y0 {
		t.Fatal("reset held", got, e, rc(y))
	}
	astTestMust(t, m.ResetRefHeld(ref))
	// last references: held and checked operations release the same way.
	survives := func(set func(ASTRef, ASTHandle) error, reset func(ASTRef) error) [2]bool {
		var out [2]bool
		a, e := m.MkAnd(x, y)
		astTestMust(t, e)
		astTestMust(t, set(ref, a))
		astTestMust(t, set(ref, x))
		_, e = m.GetRefCount(a)
		out[0] = e == nil
		c, e := m.MkOr(x, y)
		astTestMust(t, e)
		astTestMust(t, set(ref, c))
		astTestMust(t, reset(ref))
		_, e = m.GetRefCount(c)
		out[1] = e == nil
		return out
	}
	checked := survives(m.SetRef, m.ResetRef)
	held := survives(m.SetRefHeld, m.ResetRefHeld)
	if checked != held || rc(x) != x0 || rc(y) != y0 {
		t.Fatal("held release differs", checked, held, rc(x), rc(y))
	}
	astTestMust(t, m.DelRef(&ref))
}
