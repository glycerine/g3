package ast

import (
	"errors"
	"testing"
)

// The batched releases are loops of DecRef: same order, same deletions,
// and the same stop point on an invalid handle.
func TestASTDecRefEachOrderAndFailure(t *testing.T) {
	m := astTestManager(t)
	b, e := m.MkBoolSort()
	astTestMust(t, e)
	c, e := m.MkConst(astTestSymbol(t, m, "batch-c"), b)
	astTestMust(t, e)
	astTestPin(t, m, c)
	f, e := m.MkFuncDecl(astTestSymbol(t, m, "batch-f"), []ASTHandle{b}, b, ASTFuncDeclInfo{})
	astTestMust(t, e)
	astTestPin(t, m, f)
	rc := func(h ASTHandle) uint32 {
		t.Helper()
		n, err := m.GetRefCount(h)
		astTestMust(t, err)
		return n
	}
	base := rc(c)

	// Releasing fc (last reference) before c deletes fc, which drops its
	// own reference to c; the reverse batch must do that first.
	fc, e := m.MkApp(f, []ASTHandle{c})
	astTestMust(t, e)
	astTestMust(t, m.IncRef(fc))
	astTestMust(t, m.IncRef(c))
	if rc(c) != base+2 {
		t.Fatal("setup refcount", rc(c), base)
	}
	n, e := m.DecRefEachReverse([]ASTHandle{c, {}, fc})
	if e != nil || n != 3 {
		t.Fatal("reverse batch", n, e)
	}
	if rc(c) != base {
		t.Fatal("reverse batch refcount", rc(c), base)
	}
	if m.IsLiveHandle(fc) {
		t.Fatal("released app still live")
	}

	// Forward batch stops at the stale handle and reports the elements
	// processed before it; later elements are untouched.
	astTestMust(t, m.IncRef(c))
	astTestMust(t, m.IncRef(c))
	n, e = m.DecRefEach([]ASTHandle{c, fc, c})
	if !errors.Is(e, ErrASTHandle) || n != 1 {
		t.Fatal("forward batch failure", n, e)
	}
	if rc(c) != base+1 {
		t.Fatal("forward batch refcount", rc(c), base)
	}
	n, e = m.DecRefEachReverse([]ASTHandle{fc, c})
	if !errors.Is(e, ErrASTHandle) || n != 1 {
		t.Fatal("reverse batch failure", n, e)
	}
	if rc(c) != base {
		t.Fatal("reverse batch failure refcount", rc(c), base)
	}
	if n, e = m.DecRefEach(nil); n != 0 || e != nil {
		t.Fatal("empty batch", n, e)
	}
}
