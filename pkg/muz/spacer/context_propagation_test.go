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

func TestSpacerPropagationContinuesAfterPredicateFails(t *testing.T) {
	f := newContextAnswerFixture(t, false)
	c, m := f.context, f.context.manager
	falsehood, err := m.MkFalse()
	if err != nil {
		t.Fatal(err)
	}
	// The first predicate has a fact, so false is not inductive. The second
	// has no initial or transition states, so false is inductive for it.
	second, err := NewPredicateTransformer(c.owner, "propagate-empty", falsehood, falsehood, c.config)
	if err != nil {
		t.Fatal(err)
	}
	c.transformers[f.other] = second
	c.order = append(c.order, f.other)
	first := c.transformers[f.query]
	for _, pt := range []*PredicateTransformer{first, second} {
		if added, err := pt.AddLemma(falsehood, 0); err != nil || !added {
			t.Fatal("add level-zero lemma", added, err)
		}
	}
	c.inductive = 7
	converged, err := c.propagate(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if converged || c.inductive != 7 {
		t.Fatal("failed predicate must prevent convergence at level zero", converged, c.inductive)
	}
	firstZero, err := first.frames.FrameLemmas(0)
	if err != nil || len(firstZero) != 1 || !m.IsFalse(firstZero[0]) {
		t.Fatal("fact predicate's false lemma must stay at level zero", firstZero, err)
	}
	secondInfinity, err := second.frames.FrameLemmas(InfinityLevel)
	if err != nil || len(secondInfinity) != 1 || !m.IsFalse(secondInfinity[0]) {
		t.Fatal("later predicate's inductive lemma must still propagate", secondInfinity, err)
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
