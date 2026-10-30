package spacer

// Source: src/muz/spacer/spacer_context.cpp context::propagate.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSpacerPropagationRecordsActualConvergenceLevel(t *testing.T) {
	f := newContextAnswerFixture(t, true)
	c, m := f.context, f.context.manager
	falsehood, err := m.MkFalse()
	if err != nil {
		t.Fatal(err)
	}
	pt := c.transformers[f.query]
	if _, err = pt.AddLemma(falsehood, 0); err != nil {
		t.Fatal(err)
	}
	c.inductive = 7
	converged, err := c.propagate(0, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !converged || c.inductive != 0 {
		t.Fatal("propagate must record the convergence level instead of the query bound", converged, c.inductive)
	}
	lemmas, err := pt.frames.FrameLemmas(InfinityLevel)
	if err != nil || len(lemmas) != 1 || !m.IsFalse(lemmas[0]) {
		t.Fatal("converged lemma did not reach the invariant frame", lemmas, err)
	}
}

func TestSpacerPropagationSimplifiesAfterConvergenceAboveQueryBound(t *testing.T) {
	f := newContextAnswerFixture(t, true)
	c, m := f.context, f.context.manager
	boolean, err := m.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	var pins []ast.ASTRef
	defer func() {
		if err := releaseNormalizeRefs(m, pins); err != nil {
			t.Fatal(err)
		}
	}()
	own := func(h ast.ASTHandle, err error) ast.ASTHandle {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		ref, err := m.NewRef(h)
		if err != nil {
			t.Fatal(err)
		}
		pins = append(pins, ref)
		return h
	}
	a := own(m.MkFreshConst("propagate-post-a", boolean))
	b := own(m.MkFreshConst("propagate-post-b", boolean))
	weak := own(m.MkOr(a, b))
	pt := c.transformers[f.query]
	for _, formula := range []ast.ASTHandle{weak, a} {
		if _, err = pt.AddLemma(formula, InfinityLevel); err != nil {
			t.Fatal(err)
		}
	}
	c.simplifyPost = true
	c.inductive = 7
	converged, err := c.propagate(1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if converged || c.inductive != 7 {
		t.Fatal("convergence above the query bound must not report an inductive query", converged, c.inductive)
	}
	lemmas, err := pt.frames.FrameLemmas(InfinityLevel)
	if err != nil || len(lemmas) != 1 || lemmas[0] != a {
		t.Fatal("post-propagation simplification did not remove the subsumed invariant", lemmas, err)
	}
}
