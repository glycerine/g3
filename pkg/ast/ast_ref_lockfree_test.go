package ast

import (
	"errors"
	"testing"
)

// TestASTSetRefLockFreeMatchesLocked runs the same reference assignments
// through SetRef/RefGet (lock-free fast path in normal builds) and through
// their locked paths, and compares counts, survivals and errors.
func TestASTSetRefLockFreeMatchesLocked(t *testing.T) {
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	x, e := m.MkConst(astTestSymbol(t, m, "lf-x"), b)
	astTestMust(t, e)
	astTestPin(t, m, x)
	y, e := m.MkConst(astTestSymbol(t, m, "lf-y"), b)
	astTestMust(t, e)
	astTestPin(t, m, y)
	rc := func(h ASTHandle) uint32 {
		n, e := m.GetRefCount(h)
		astTestMust(t, e)
		return n
	}
	run := func(set func(ASTRef, ASTHandle) error, get func(ASTRef) (ASTHandle, error)) []uint32 {
		var out []uint32
		ref, e := m.NewRef(ASTHandle{})
		astTestMust(t, e)
		astTestMust(t, set(ref, ASTHandle{}))
		if h, err := get(ref); err != nil || !h.IsNull() {
			t.Fatal("empty reference", h, err)
		}
		if err := set(ref, ASTHandle{origin: m.origin}); !errors.Is(err, ErrASTHandle) {
			t.Fatal("malformed null handle", err)
		}
		astTestMust(t, set(ref, x))
		astTestMust(t, set(ref, x))
		out = append(out, rc(x), rc(y))
		astTestMust(t, set(ref, y))
		out = append(out, rc(x), rc(y))
		if got, e := get(ref); e != nil || got != y {
			t.Fatal("get", got, e)
		}
		a, e := m.MkAnd(x, y)
		astTestMust(t, e)
		astTestMust(t, set(ref, a))
		astTestMust(t, set(ref, x)) // releases a's last reference
		_, e = m.GetRefCount(a)
		if e == nil {
			out = append(out, 1)
		} else {
			out = append(out, 0)
		}
		out = append(out, rc(x), rc(y))
		astTestMust(t, m.ResetRef(ref))
		stale := ref
		astTestMust(t, m.DelRef(&ref))
		if err := set(stale, ASTHandle{}); !errors.Is(err, ErrASTHandle) {
			t.Fatal("clearing stale reference", err)
		}
		if e := set(stale, x); !errors.Is(e, ErrASTHandle) {
			t.Fatal("stale set", e)
		}
		if _, e := get(stale); !errors.Is(e, ErrASTHandle) {
			t.Fatal("stale get", e)
		}
		out = append(out, rc(x), rc(y))
		return out
	}
	fast := run(m.SetRef, m.RefGet)
	locked := run(m.setRefChecked, m.refGetChecked)
	if len(fast) != len(locked) {
		t.Fatal(fast, locked)
	}
	for i := range fast {
		if fast[i] != locked[i] {
			t.Fatal("lock-free and locked SetRef differ", fast, locked)
		}
	}
}
