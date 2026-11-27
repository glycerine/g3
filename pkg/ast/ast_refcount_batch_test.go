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

// Both paths retain the same four references to an already-live sort.
func BenchmarkASTSharedReferenceBatch(b *testing.B) {
	m := astTestManager(b)
	sort, err := m.MkBoolSort()
	astTestMust(b, err)
	astTestPin(b, m, sort)
	refs := []ASTHandle{sort, sort, sort, sort}
	b.Run("Individual", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			for _, h := range refs {
				if err := m.IncRef(h); err != nil {
					b.Fatal(err)
				}
			}
			for _, h := range refs {
				if consumed, err := m.DecRefConsumed(h); !consumed || err != nil {
					b.Fatal(consumed, err)
				}
			}
		}
	})
	b.Run("Combined", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if inc, dec, err := m.IncRefThenDecRef(refs, refs); inc != len(refs) || dec != len(refs) || err != nil {
				b.Fatal(inc, dec, err)
			}
		}
	})
}

func TestASTDecRefConsumedRejectsMalformedHandles(t *testing.T) {
	m := astTestManager(t)
	h, err := m.MkBoolSort()
	astTestMust(t, err)
	astTestPin(t, m, h)
	n, err := m.GetRefCount(h)
	astTestMust(t, err)
	for _, bad := range []ASTHandle{
		{origin: m.origin},
		{generation: 1},
		{origin: h.origin + 1, slot: h.slot, generation: h.generation},
		{origin: h.origin, slot: h.slot, generation: h.generation + 1},
		{origin: h.origin, slot: m.nextNode, generation: h.generation},
	} {
		if consumed, err := m.DecRefConsumed(bad); consumed || err != ErrASTHandle {
			t.Fatalf("release of %v returned %v, %v", bad, consumed, err)
		}
	}
	if consumed, err := m.DecRefConsumed(ASTHandle{}); !consumed || err != nil {
		t.Fatalf("null release returned %v, %v", consumed, err)
	}
	if after, err := m.GetRefCount(h); after != n || err != nil {
		t.Fatalf("live count after refused releases = %d, want %d, error %v", after, n, err)
	}
}
