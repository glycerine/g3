package rewriter

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestRewriterNativeResultAndProofRootsGrowth(t *testing.T) {
	m := astTestManager(t)
	satOK(t, m.ToggleProofMode(ast.ASTProofEnabled))
	p := goalTestAtom(t, m, "native-result-root")
	pr, e := m.MkProofHypothesis(p)
	satOK(t, e)
	astTestPin(t, m, pr)
	c, e := newRewriterCore(m, true)
	satOK(t, e)
	defer func() { satOK(t, c.Close()) }()
	beforeP, e := m.GetRefCount(p)
	satOK(t, e)
	beforeProof, e := m.GetRefCount(pr)
	satOK(t, e)
	used := m.Budget().Used()
	m.Budget().FailAfter(0)
	defer m.Budget().FailAfter(-1)
	proofs := uint32(0)
	for i := 0; i < 4097; i++ {
		satOK(t, appendRewriterRoot(m, &c.results, p))
		h := ast.ASTHandle{}
		if i%3 != 0 {
			h = pr
			proofs++
		}
		satOK(t, appendRewriterRoot(m, &c.proofResults, h))
	}
	count, e := m.GetRefCount(p)
	satOK(t, e)
	if count != beforeP+4097 {
		t.Fatal("result references", count, beforeP)
	}
	count, e = m.GetRefCount(pr)
	satOK(t, e)
	if count != beforeProof+proofs {
		t.Fatal("proof references", count, beforeProof, proofs)
	}
	satOK(t, c.elimReflexProofs(1))
	if len(c.proofResults) != int(proofs)+1 || !c.proofResults[0].IsNull() {
		t.Fatal("proof prefix/compaction", len(c.proofResults), proofs)
	}
	count, e = m.GetRefCount(pr)
	satOK(t, e)
	if count != beforeProof+proofs {
		t.Fatal("compaction changed ownership", count, beforeProof, proofs)
	}
	satOK(t, shrinkRewriterRoots(m, &c.results, 7))
	count, e = m.GetRefCount(p)
	satOK(t, e)
	if count != beforeP+7 {
		t.Fatal("preserved result prefix", count, beforeP)
	}
	satOK(t, shrinkRewriterRoots(m, &c.results, 0))
	satOK(t, shrinkRewriterRoots(m, &c.proofResults, 0))
	count, e = m.GetRefCount(p)
	satOK(t, e)
	if count != beforeP {
		t.Fatal("result release", count, beforeP)
	}
	count, e = m.GetRefCount(pr)
	satOK(t, e)
	if count != beforeProof {
		t.Fatal("proof release", count, beforeProof)
	}
	if m.Budget().Used() != used {
		t.Fatal("native root backing charged arena")
	}
}

func TestRewriterNativeRootOnlyReferenceAliasReplacement(t *testing.T) {
	m := astTestManager(t)
	sort, e := m.MkBoolSort()
	satOK(t, e)
	h, e := m.MkVar(201, sort)
	satOK(t, e)
	var roots []ast.ASTHandle
	satOK(t, appendRewriterRoot(m, &roots, h))
	satOK(t, replaceRewriterRoot(m, &roots, 0, h))
	count, e := m.GetRefCount(h)
	satOK(t, e)
	if count != 1 {
		t.Fatal("alias replacement owns one reference", count)
	}
	satOK(t, shrinkRewriterRoots(m, &roots, 0))
	if m.IsLiveHandle(h) {
		t.Fatal("last root did not delete AST")
	}
	// A repeated reset must not decrement the already-consumed reference.
	satOK(t, shrinkRewriterRoots(m, &roots, 0))
}

func TestRewriterNativeFrameAndScopeGrowth(t *testing.T) {
	m := astTestManager(t)
	p := goalTestAtom(t, m, "native-frame-root")
	c, e := newRewriterCore(m, false)
	satOK(t, e)
	defer func() { satOK(t, c.Close()) }()
	// Cache objects have actual owning headers. Construct these first so this
	// test independently disables reservations during cold scope/frame growth.
	for i := 0; i < 65; i++ {
		_, e = c.pushCache(&c.cacheStack, 99)
		satOK(t, e)
	}
	used := m.Budget().Used()
	before, e := m.GetRefCount(p)
	satOK(t, e)
	m.Budget().FailAfter(0)
	defer m.Budget().FailAfter(-1)
	for i := 0; i < 4097; i++ {
		satOK(t, c.pushFrameCore(p, false, uint32(i), 3))
	}
	for i := 0; i < 65; i++ {
		c.root = p
		c.numQVars = uint32(i)
		satOK(t, c.beginScope())
	}
	for i := 64; i >= 0; i-- {
		satOK(t, c.endScope())
		if c.root != p || c.numQVars != uint32(i) {
			t.Fatal("scope restoration", i, c.numQVars)
		}
	}
	if len(c.frames) != 4097 || len(c.scopes) != 0 || c.cache != c.cacheStack[0] {
		t.Fatal("native traversal storage")
	}
	for i, fr := range c.frames {
		if fr.curr != p || fr.state() != uint32(i)&3 || fr.maxDepth() != 3 {
			t.Fatal("packed frame state", i)
		}
	}
	after, e := m.GetRefCount(p)
	satOK(t, e)
	if after != before || m.Budget().Used() != used {
		t.Fatal("borrowed frame/scope storage changed owners or arena charge", after, before)
	}
}

func TestVarSubstitutionNativeColdBindingGrowth(t *testing.T) {
	m := astTestManager(t)
	a, b := goalTestAtom(t, m, "native-binding-a"), goalTestAtom(t, m, "native-binding-b")
	variable := shiftTestVar(t, m, 0)
	out := astTestPin(t, m, ast.ASTHandle{})
	args := make([]ast.ASTHandle, 4097)
	for i := range args {
		args[i] = a
	}
	args[len(args)-1] = b
	for _, standard := range []bool{false, true} {
		s, e := NewVarSubstitution(m, standard)
		satOK(t, e)
		used := m.Budget().Used()
		m.Budget().FailAfter(0)
		e = s.Substitute(variable, args, out)
		m.Budget().FailAfter(-1)
		satOK(t, e)
		got, e := m.RefGet(out)
		satOK(t, e)
		want := a
		if standard {
			want = b
		}
		if got != want || m.Budget().Used() != used {
			t.Fatal("native binding order or backing charge", standard, got, want)
		}
		satOK(t, s.Reset())
		satOK(t, s.Close())
	}
}
