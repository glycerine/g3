package ast

import (
	"slices"
	"testing"
)

func TestIsolatedCheckRestoresIDState(t *testing.T) {
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	mk := func(name string) ASTHandle {
		c, e := m.MkConst(astTestSymbol(t, m, name), b)
		astTestMust(t, e)
		return c
	}
	astTestPin(t, m, mk("a"))
	// Leave released IDs on the free list.
	for _, name := range []string{"x", "y"} {
		r, e := m.NewRef(mk(name))
		astTestMust(t, e)
		astTestMust(t, m.DelRef(&r))
	}
	exprs, decls, fresh := m.ExprIDs.Snapshot(), m.declIDs.Snapshot(), m.freshID
	restored, err := m.IsolatedCheck(func() error {
		c, e := m.MkFreshConst("k", b)
		if e != nil {
			return e
		}
		r1, e := m.NewRef(c)
		if e != nil {
			return e
		}
		n, e := m.MkNot(c)
		if e != nil {
			return e
		}
		r2, e := m.NewRef(n)
		if e != nil {
			return e
		}
		// Release in creation order, so the free list order changes.
		if e = m.DelRef(&r1); e != nil {
			return e
		}
		return m.DelRef(&r2)
	})
	astTestMust(t, err)
	if !restored || m.ExprIDs.Range() != exprs.Range() || !slices.Equal(m.ExprIDs.FreeIDs(), exprs.FreeIDs()) ||
		m.declIDs.Range() != decls.Range() || !slices.Equal(m.declIDs.FreeIDs(), decls.FreeIDs()) || m.freshID != fresh {
		t.Fatal("isolated check changed the ID state", restored, m.ExprIDs.FreeIDs(), exprs.FreeIDs(), m.freshID, fresh)
	}

	// A check that leaves a term alive is reported, and the IDs stay.
	var keep ASTRef
	restored, err = m.IsolatedCheck(func() (e error) {
		keep, e = m.NewRef(mk("z"))
		return e
	})
	astTestMust(t, err)
	if restored {
		t.Fatal("isolated check restored IDs of a live term")
	}
	astTestMust(t, m.DelRef(&keep))
}
