package rewriter

// Source: src/ast/rewriter/rewriter.cpp:23-204;
// src/ast/rewriter/rewriter.h:31-119.

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestRewriterCoreOriginalScopedCachesAndRootRestoration(t *testing.T) {
	for _, proof := range []bool{false, true} {
		t.Run(fmt.Sprint(proof), func(t *testing.T) {
			m := astTestManager(t)
			satOK(t, m.ToggleProofMode(ast.ASTProofEnabled))
			p, q, r := goalTestAtom(t, m, "core-p"), goalTestAtom(t, m, "core-q"), goalTestAtom(t, m, "core-r")
			pr, e := m.MkProofHypothesis(p)
			satOK(t, e)
			astTestPin(t, m, pr)
			c, e := newRewriterCore(m, proof)
			satOK(t, e)
			defer func() { satOK(t, c.Close()) }()
			if !c.cancelCheck || c.numQVars != 0 || len(c.cacheStack) != 1 || len(c.proofCacheStack) != map[bool]int{false: 0, true: 1}[proof] {
				t.Fatal("source core constructor")
			}
			satOK(t, c.SetCancelCheck(false))
			var counts [4]uint32
			for i, h := range []ast.ASTHandle{p, q, r, pr} {
				counts[i], e = m.GetRefCount(h)
				satOK(t, e)
			}
			cache := func(k, v ast.ASTHandle) {
				if proof {
					satOK(t, c.cacheResultProof(k, v, pr))
				} else {
					satOK(t, c.cacheResult(k, v))
				}
			}
			c.root = p
			c.numQVars = 7
			cache(p, q)
			base := c.cache
			satOK(t, c.beginScope())
			c.root = q
			c.numQVars = 9
			cache(p, r)
			first := c.cache
			satOK(t, c.beginScope())
			c.root = r
			c.numQVars = 11
			cache(q, p)
			satOK(t, c.beginScope())
			cache(r, q)
			if len(c.cacheStack) != 4 {
				t.Fatal("source scope cache stack growth")
			}
			satOK(t, c.endScope())
			if c.root != r || c.numQVars != 11 {
				t.Fatal("nested scope restored root/count")
			}
			satOK(t, c.endScope())
			if c.root != q || c.numQVars != 9 || c.cache != first {
				t.Fatal("first scope restored")
			}
			got, e := c.getCached(p)
			satOK(t, e)
			if got != r {
				t.Fatal("inner cache survived deeper scopes")
			}
			satOK(t, c.endScope())
			if c.root != p || c.numQVars != 7 || c.cache != base {
				t.Fatal("base scope restored")
			}
			got, e = c.getCached(p)
			satOK(t, e)
			if got != q {
				t.Fatal("base cache isolation")
			}
			if proof {
				got, e = c.getCachedProof(p)
				satOK(t, e)
				if got != pr {
					t.Fatal("parallel proof cache")
				}
			}
			satOK(t, c.beginScope())
			if c.cache != first {
				t.Fatal("source reuse of existing cache owner")
			}
			size, e := c.getCacheSize()
			satOK(t, e)
			if size != 0 {
				t.Fatal("reused scope starts empty")
			}
			satOK(t, c.endScope())
			satOK(t, c.Reset())
			if !c.root.IsNull() || c.numQVars != 0 || len(c.scopes) != 0 || c.cache != base {
				t.Fatal("source reset state")
			}
			if c.cancelCheck {
				t.Fatal("reset changed cancellation setting")
			}
			satOK(t, c.Close())
			for i, h := range []ast.ASTHandle{p, q, r, pr} {
				n, e := m.GetRefCount(h)
				satOK(t, e)
				if n != counts[i] {
					t.Fatal("scoped cache teardown leaked AST ownership", i, n, counts[i])
				}
			}
		})
	}
}

func TestRewriterCoreSharedFramesQuantifierChildrenAndPackedFields(t *testing.T) {
	m := astTestManager(t)
	c, e := newRewriterCore(m, false)
	satOK(t, e)
	defer func() { satOK(t, c.Close()) }()
	p, q := goalTestAtom(t, m, "frame-p"), goalTestAtom(t, m, "frame-q")
	app := goalTestApp(t, m, ast.ASTOpOr, p, q)
	shared, e := c.mustCache(app)
	satOK(t, e)
	if shared {
		t.Fatal("single reference application cached")
	}
	extra := astTestPin(t, m, app)
	shared, e = c.mustCache(app)
	satOK(t, e)
	if !shared {
		t.Fatal("shared nonconstant not cached")
	}
	c.root = app
	shared, e = c.mustCache(app)
	satOK(t, e)
	if shared {
		t.Fatal("root cached")
	}
	c.root = p
	shared, e = c.mustCache(p)
	satOK(t, e)
	if shared {
		t.Fatal("constant cached")
	}
	satOK(t, appendRewriterRoot(m, &c.results, q))
	satOK(t, c.pushFrameCore(app, true, 6, 7, ast.ASTApp, 0))
	fr := &c.frames[0]
	if fr.state() != 2 || fr.maxDepth() != 3 || fr.index() != 0 || fr.spos != 1 || !fr.cacheResult() || fr.newChild() {
		t.Fatal("source frame bitfield construction")
	}
	fr.setIndex((1 << 26) - 1)
	fr.setIndex(fr.index() + 1)
	if fr.index() != 0 {
		t.Fatal("source 26-bit child index wrap")
	}
	yes, e := c.isChildOfTopFrame(p)
	satOK(t, e)
	if !yes {
		t.Fatal("direct application child")
	}
	yes, e = c.isChildOfTopFrame(app)
	satOK(t, e)
	if yes {
		t.Fatal("parent is not its own child")
	}
	satOK(t, c.setNewChildIfDifferent(p, p))
	if fr.newChild() {
		t.Fatal("identical child set modified flag")
	}
	satOK(t, c.setNewChildIfDifferent(p, q))
	if !fr.newChild() {
		t.Fatal("changed child not marked")
	}
	satOK(t, m.ResetRef(extra))
	// Quantifier children include body then patterns then no-patterns.
	b, e := m.MkBoolSort()
	satOK(t, e)
	pat, e := m.MkFamilyApp(2, 0, nil, []ast.ASTHandle{app})
	satOK(t, e)
	astTestPin(t, m, pat)
	quant, e := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: app, Patterns: []ast.ASTHandle{pat}})
	satOK(t, e)
	astTestPin(t, m, quant)
	c.frames = c.frames[:0]
	satOK(t, c.pushFrameCore(quant, false, 0, 3, ast.ASTQuantifier, 0))
	for _, child := range []ast.ASTHandle{app, pat} {
		yes, e = c.isChildOfTopFrame(child)
		satOK(t, e)
		if !yes {
			t.Fatal("source quantifier child", child)
		}
	}
	yes, e = c.isChildOfTopFrame(p)
	satOK(t, e)
	if yes {
		t.Fatal("grandchild accepted")
	}
	c.frames = c.frames[:0]
	noPat, e := m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: []ast.ASTHandle{b}, Names: []usymbol.Symbol{usymbol.Numerical(0)}, Body: app, NoPatterns: []ast.ASTHandle{q}})
	satOK(t, e)
	astTestPin(t, m, noPat)
	satOK(t, c.pushFrameCore(noPat, false, 0, 3, ast.ASTQuantifier, 0))
	yes, e = c.isChildOfTopFrame(q)
	satOK(t, e)
	if !yes {
		t.Fatal("source no-pattern child")
	}
	astTestPin(t, m, noPat)
	shared, e = c.mustCache(noPat)
	satOK(t, e)
	if !shared {
		t.Fatal("shared quantifier not cached")
	}
	c.frames = c.frames[:0]
	for i := 0; i < 40; i++ {
		satOK(t, c.pushFrameCore(app, i%2 == 0, uint32(i), uint32(i), ast.ASTApp, 0))
	}
	capacity := cap(c.frames)
	satOK(t, c.Reset())
	if len(c.frames) != 0 || cap(c.frames) != capacity {
		t.Fatal("reset retains frame storage")
	}
	satOK(t, c.Cleanup())
	if c.frames != nil || c.scopes != nil || len(c.cacheStack) != 1 {
		t.Fatal("cleanup frees frame/scope storage and recreates base cache")
	}
}

func TestRewriterCoreProofCompactionResetAndCleanupSourceAsymmetry(t *testing.T) {
	m := astTestManager(t)
	satOK(t, m.ToggleProofMode(ast.ASTProofEnabled))
	p, q := goalTestAtom(t, m, "proof-core-p"), goalTestAtom(t, m, "proof-core-q")
	a, e := m.MkProofHypothesis(p)
	satOK(t, e)
	astTestPin(t, m, a)
	b, e := m.MkProofHypothesis(q)
	satOK(t, e)
	astTestPin(t, m, b)
	c, e := newRewriterCore(m, true)
	satOK(t, e)
	defer func() { satOK(t, c.Close()) }()
	for _, pr := range []ast.ASTHandle{{}, a, {}, b, {}} {
		satOK(t, appendRewriterRoot(m, &c.proofResults, pr))
	}
	satOK(t, c.elimReflexProofs(1))
	n := uint32(len(c.proofResults))
	if n != 3 {
		t.Fatal("source stable proof compaction", n)
	}
	for i, want := range []ast.ASTHandle{{}, a, b} {
		got := c.proofResults[i]
		if got != want {
			t.Fatal("compaction preserves prefix and order", i, got, want)
		}
	}
	satOK(t, appendRewriterRoot(m, &c.results, p))
	// Original free_memory finalizes results but leaves proofResults untouched.
	satOK(t, c.Cleanup())
	n = uint32(len(c.proofResults))
	if n != 3 {
		t.Fatal("source cleanup proof-stack asymmetry", n)
	}
	n = uint32(len(c.results))
	if n != 0 {
		t.Fatal("cleanup result stack not finalized")
	}
	satOK(t, c.Reset())
	n = uint32(len(c.proofResults))
	if n != 0 {
		t.Fatal("proof-enabled reset clears proof stack")
	}
	// Reset clears only the base cache, not retained higher scope caches.
	c.root = p
	satOK(t, c.beginScope())
	satOK(t, c.cacheResultProof(p, q, a))
	inner := c.cache
	satOK(t, c.Reset())
	size, e := inner.Size()
	satOK(t, e)
	if size != 1 {
		t.Fatal("source reset unexpectedly cleared non-base cache", size)
	}
	copyOwner := *c
	if e = copyOwner.Reset(); e != ast.ErrASTHandle {
		t.Fatal("copied core owner", e)
	}
	satOK(t, c.Close())
	if e = c.Reset(); e != ast.ErrASTHandle {
		t.Fatal("closed core owner", e)
	}
}

func TestRewriterCoreNativeAndDebugAssertionBoundaries(t *testing.T) {
	m := astTestManager(t)
	c, e := newRewriterCore(m, false)
	satOK(t, e)
	defer func() { satOK(t, c.Close()) }()
	p, q, r := goalTestAtom(t, m, "assert-core-p"), goalTestAtom(t, m, "assert-core-q"), goalTestAtom(t, m, "assert-core-r")
	app := goalTestApp(t, m, ast.ASTOpOr, p, q)
	satOK(t, appendRewriterRoot(m, &c.proofResults, ast.ASTHandle{}))
	e = c.elimReflexProofs(0)
	if ast.ASTSourceDebug {
		if e != ast.ErrASTDomain {
			t.Fatal("source proof generation assertion", e)
		}
	} else {
		satOK(t, e)
	}
	satOK(t, c.Reset())
	n := uint32(len(c.proofResults))
	want := uint32(0)
	if ast.ASTSourceDebug {
		want = 1
	}
	if n != want {
		t.Fatal("no-proof reset leaves source proof stack alone", n, want)
	}
	satOK(t, c.pushFrameCore(app, false, 0, 3, ast.ASTApp, 0))
	e = c.setNewChild(r)
	if ast.ASTSourceDebug {
		if e != ast.ErrASTDomain || c.frames[0].newChild() {
			t.Fatal("source direct-child assertion", e)
		}
	} else {
		satOK(t, e)
		if !c.frames[0].newChild() {
			t.Fatal("native source flag write")
		}
	}
}
