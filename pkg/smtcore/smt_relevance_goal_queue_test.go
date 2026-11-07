package smtcore

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func relevanceGoalFixture(t *testing.T) (*ast.ASTManager, *SMTInternalizerState, *SMTRelevancyPropagator, *SMTActivityCaseSplitQueue) {
	t.Helper()
	m := astTestManager(t)
	_, p, s := smtInternalizerFixture(t, m, nil)
	q, err := NewSMTActivityCaseSplitQueue(s)
	astTestMust(t, err)
	astTestMust(t, q.ConfigurePhases(SMTParamsState{CaseSplitStrategy: 5, RelevancyLvl: 2}))
	return m, s, p, q
}

func TestSMTRelevanceGoalCachesGenerationAtAdmission(t *testing.T) {
	m, s, p, q := relevanceGoalFixture(t)
	astTestMust(t, s.SetGeneration(40))
	astTestMust(t, s.beginSearchPersistence(0, SMTParamsState{}))
	t.Cleanup(func() { astTestMust(t, s.endSearchPersistence()) })
	if s.generation != 0 {
		t.Fatal("source init_search_eh did not reset global generation")
	}
	var terms []ast.ASTHandle
	for i, gen := range []uint32{9, 2} {
		astTestMust(t, s.SetGeneration(gen))
		x := smtRelevancyBool(t, m, fmt.Sprintf("goal-cache-%d", i))
		astTestMust(t, s.Internalize(x, true))
		_, err := s.MkEnode(x, false, false, true)
		astTestMust(t, err)
		astTestMust(t, p.MarkRelevant(x))
		terms = append(terms, x)
	}
	var trail SMTEnodeTrail
	astTestMust(t, s.context.enodes.get(terms[0]).SetGeneration(&trail, 0))
	defer func() { astTestMust(t, trail.Undo(0)) }()
	want, _ := s.BoolVar(terms[1])
	for i := 0; i < 2; i++ {
		got, _, err := q.NextCaseSplit()
		astTestMust(t, err)
		if got != want || !s.relevanceGoal.heap.Contains(1) || s.relevanceGoal.second[1].lastDecided != -1 {
			t.Fatal("selection must use cached generation and reinsert successful candidate", got, want)
		}
	}
	if s.relevanceGoal.second[0].generation != 9 || s.relevanceGoal.second[1].generation != 2 {
		t.Fatal("live enode mutation rewrote admitted generations")
	}
}

func TestSMTRelevanceGoalConsumedEntryScopeReinsertionAndDeletion(t *testing.T) {
	m, s, p, q := relevanceGoalFixture(t)
	a := smtRelevancyBool(t, m, "goal-retained")
	astTestMust(t, s.Internalize(a, true))
	astTestMust(t, p.MarkRelevant(a))
	v, _ := s.BoolVar(a)
	astTestMust(t, s.PushScope())
	astTestMust(t, s.AssignLiteral(NewSMTLiteral(v, false), NewSMTBooleanAxiom()))
	got, _, err := q.NextCaseSplit()
	astTestMust(t, err)
	if got != SMTNullBoolVar || s.relevanceGoal.heap.Contains(0) || s.relevanceGoal.second[0].lastDecided != 1 {
		t.Fatal("exhausted entry decision level", got)
	}
	b := smtRelevancyBool(t, m, "goal-child")
	astTestMust(t, s.Internalize(b, true))
	astTestMust(t, p.MarkRelevant(b))
	if !s.relevanceGoal.heap.Contains(1) {
		t.Fatal("child entry absent")
	}
	astTestMust(t, s.PopScopes(1))
	if len(s.relevanceGoal.second) != 1 || !s.relevanceGoal.heap.Contains(0) || s.relevanceGoal.heap.Contains(1) || s.relevanceGoal.second[0].lastDecided != -1 {
		t.Fatal("source pop reinsert/remove sequence")
	}
	got, _, err = q.NextCaseSplit()
	astTestMust(t, err)
	if got != v {
		t.Fatal(got, v)
	}
}

func TestSMTRelevanceGoalAssignmentCallbackAndScopeRestoresGoal(t *testing.T) {
	m, s, _, _ := relevanceGoalFixture(t)
	label, err := m.MkLabelLit([]usymbol.Symbol{astTestSymbol(t, m, "goal-label")})
	astTestMust(t, err)
	body := smtRelevancyBool(t, m, "goal-body")
	goal, err := m.MkAnd(body, label)
	astTestMust(t, err)
	astTestMust(t, s.Internalize(goal, true))
	v, _ := s.BoolVar(goal)
	astTestMust(t, s.PushScope())
	astTestMust(t, s.AssignLiteral(NewSMTLiteral(v, false), NewSMTBooleanAxiom()))
	if s.relevanceGoal.current != goal {
		t.Fatal("actual assign_lit_eh did not select labeled goal")
	}
	astTestMust(t, s.PopScopes(1))
	if !s.relevanceGoal.current.IsNull() {
		t.Fatal("popped goal retained")
	}
}

func TestSMTRelevanceGoalExactDisplayAndWriterPrefixes(t *testing.T) {
	m, s, p, _ := relevanceGoalFixture(t)
	a := smtRelevancyBool(t, m, "goal-secondary-hidden")
	b := smtRelevancyBool(t, m, "goal-primary")
	astTestMust(t, s.Internalize(a, true))
	astTestMust(t, s.Internalize(b, true))
	astTestMust(t, p.MarkRelevant(a))
	var out bytes.Buffer
	astTestMust(t, s.DisplayRelevanceGoalQueue(&out))
	if out.String() != "case-splits:\n" {
		t.Fatal("secondary rendering is commented in native", out.String())
	}
	astTestMust(t, s.beginSearchPersistence(0, SMTParamsState{}))
	t.Cleanup(func() { astTestMust(t, s.endSearchPersistence()) })
	astTestMust(t, p.MarkRelevant(b))
	id, err := m.GetASTID(b)
	astTestMust(t, err)
	want := fmt.Sprintf("case-splits:\n[HEAD1]=> #%d \n", id)
	out.Reset()
	astTestMust(t, s.DisplayRelevanceGoalQueue(&out))
	if out.String() != want {
		t.Fatal(out.String(), want)
	}
	before := m.Stats()
	for cut := 0; cut < len(want); cut++ {
		w := &smtLRAOutputPrefix{remaining: cut}
		if err := s.DisplayRelevanceGoalQueue(w); !errors.Is(err, io.ErrClosedPipe) || w.text.String() != want[:cut] {
			t.Fatal(cut, err, w.text.String(), want[:cut])
		}
	}
	if err := s.DisplayRelevanceGoalQueue(smtLRAShortOutput{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(err)
	}
	if before != m.Stats() || s.relevanceGoal.head != 0 {
		t.Fatal("display mutated owner or cursor")
	}
}

func TestSMTRelevanceGoalConstructorRefusalPreservesGeneration(t *testing.T) {
	m := astTestManager(t)
	_, _, s := smtInternalizerFixture(t, m, nil)
	q, err := NewSMTActivityCaseSplitQueue(s)
	astTestMust(t, err)
	astTestMust(t, s.SetGeneration(7))
	params := SMTParamsState{CaseSplitStrategy: 5, RelevancyLvl: 2}
	m.Budget().FailAfter(0)
	err = q.ConfigurePhases(params)
	m.Budget().FailAfter(-1)
	if err == nil || s.relevanceGoal.heap != nil || s.generation != 7 {
		t.Fatal("failed constructor published owner or ran body", err, s.generation)
	}
	astTestMust(t, q.ConfigurePhases(params))
	if s.relevanceGoal.heap == nil || s.generation != 0 {
		t.Fatal("source constructor reset missing", s.generation)
	}
}
