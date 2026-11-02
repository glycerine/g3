package spacer

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func TestSpacerGeneralizerStatisticsFollowSourceOwners(t *testing.T) {
	f := newContextAnswerFixture(t, false)
	c, m := f.context, f.context.manager
	generalize := func() {
		t.Helper()
		p := c.transformers[f.query]
		falsehood, err := m.MkFalse()
		depthMust(t, err)
		obligation, err := p.NewProofObligation(nil, 1, 0, falsehood, nil)
		depthMust(t, err)
		lemma, err := NewLemmaFromProofObligation(obligation, []ast.ASTHandle{falsehood}, 1)
		depthMust(t, err)
		defer func() { depthMust(t, lemma.Close()) }()
		generalized, err := p.generalizeInductiveLemma(obligation, lemma)
		depthMust(t, err)
		if generalized != lemma {
			depthMust(t, generalized.Close())
		}
	}
	generalize()
	if got := spacerContextStatistic(t, c, "bool inductive gen"); got != 1 {
		t.Fatal("expected one generalizer call", got)
	}
	truth, err := m.MkTrue()
	depthMust(t, err)
	depthMust(t, c.AddCover(0, f.other, truth))
	if got := spacerContextStatistic(t, c, "bool inductive gen"); got != 1 {
		t.Fatal("adding a transformer duplicated global generalizer statistics", got)
	}
	depthMust(t, c.Reset())
	if got := spacerContextStatistic(t, c, "bool inductive gen"); got != 1 {
		t.Fatal("context reset destroyed its generalizer owner", got)
	}
	depthMust(t, c.Initialize(f.rules))
	if got := spacerContextStatistic(t, c, "bool inductive gen"); got != 0 {
		t.Fatal("rule initialization did not recreate generalizers", got)
	}
	generalize()
	depthMust(t, c.ResetStatistics())
	if got := spacerContextStatistic(t, c, "bool inductive gen"); got != 0 {
		t.Fatal("statistics reset did not clear generalizer counters", got)
	}
	generalize()
	m.Budget().FailAfter(0)
	err = c.UpdateRules(f.rules)
	m.Budget().FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("expected failed rule installation", err)
	}
	if got := spacerContextStatistic(t, c, "bool inductive gen"); got != 0 {
		t.Fatal("failed rule installation preserved old generalizer counters", got)
	}
	depthMust(t, c.UpdateRules(f.rules))
}
