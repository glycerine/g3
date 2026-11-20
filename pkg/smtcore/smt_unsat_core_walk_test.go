package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func smtCoreWalkFixture(t *testing.T, count int) (*ast.ASTManager, *SMTInternalizerState, []SMTLiteral) {
	t.Helper()
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	literals := make([]SMTLiteral, count)
	for i := range literals {
		expression := smtRelevancyBool(t, m, fmt.Sprint("core-walk-", i))
		astTestMust(t, state.Internalize(expression, true))
		var err error
		literals[i], err = state.Literal(expression)
		astTestMust(t, err)
	}
	return m, state, literals
}

func smtCoreWalkReason(t *testing.T, literals ...SMTLiteral) SMTBooleanJustification {
	t.Helper()
	owner, err := NewSMTSimpleJustification(true, literals)
	astTestMust(t, err)
	reason, err := NewSMTBooleanJustification(owner)
	astTestMust(t, err)
	return reason
}

func smtCoreWalkResolver(t *testing.T, state *SMTInternalizerState) *SMTConflictResolver {
	t.Helper()
	r, err := NewSMTConflictResolver(state)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, r.Close()) })
	return r
}

func expectSMTRawCore(t *testing.T, got, want []SMTLiteral) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatal("raw core length", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatal("raw core discovery order/polarity", got, want)
		}
	}
}

func TestSMTUnsatCoreReverseTrailDiscoveryAndDuplicates(t *testing.T) {
	_, s, ls := smtCoreWalkFixture(t, 4)
	a, b, c, d := ls[0], ls[1], ls[2], ls[3]
	astTestMust(t, s.PushScope())
	for _, literal := range []SMTLiteral{a, b} {
		astTestMust(t, s.AssignLiteral(literal, NewSMTBooleanAxiom()))
		s.booleanData[literal.Var()].SetAssumption(true)
	}
	astTestMust(t, s.AssignLiteral(c, smtCoreWalkReason(t, b)))
	astTestMust(t, s.AssignLiteral(d, smtCoreWalkReason(t, a)))
	conflict := smtCoreWalkReason(t, c, d)
	r := smtCoreWalkResolver(t, s)
	for attempt := 0; attempt < 2; attempt++ {
		core, err := r.UnsatCore(conflict, SMTNullLiteral)
		astTestMust(t, err)
		// d is visited before c even though the conflict lists c first.
		// Discovery and later assumption visits both append raw entries.
		expectSMTRawCore(t, core, []SMTLiteral{a, b, b, a})
	}
}

func TestSMTConflictLearningRetainsAssumptionsBelowSearchDecision(t *testing.T) {
	_, s, literals := smtCoreWalkFixture(t, 3)
	a, decision, implied := literals[0], literals[1], literals[2]
	astTestMust(t, s.PushScope())
	astTestMust(t, s.AssignLiteral(a, NewSMTBooleanAxiom()))
	s.booleanData[a.Var()].SetAssumption(true)
	astTestMust(t, s.PushScope())
	astTestMust(t, s.AssignLiteral(decision, NewSMTBooleanAxiom()))
	astTestMust(t, s.AssignLiteral(implied, smtCoreWalkReason(t, a, decision)))
	r := smtCoreWalkResolver(t, s)
	result, err := r.resolve(smtCoreWalkReason(t, implied, decision), SMTNullLiteral, 1, 0)
	astTestMust(t, err)
	if !result.Resolved || result.NewScopeLevel != 1 {
		t.Fatal("resolution did not backtrack to the assumption search level", result)
	}
	expectSMTRawCore(t, result.Lemma, []SMTLiteral{decision.Negated(), a.Negated()})
	astTestMust(t, s.PopScopes(1))
	astTestMust(t, s.AddLearnedClause(result.Lemma))
	consistent, err := s.Propagate()
	astTestMust(t, err)
	if !consistent {
		t.Fatal("learned guarded literal conflicted")
	}
	core, err := r.UnsatCore(smtCoreWalkReason(t, decision.Negated()), SMTNullLiteral)
	astTestMust(t, err)
	expectSMTRawCore(t, core, []SMTLiteral{a, a})
}

func TestSMTUnsatCoreStopsBeforeOlderAssignedReasons(t *testing.T) {
	_, s, ls := smtCoreWalkFixture(t, 4)
	a, b, c, d := ls[0], ls[1], ls[2], ls[3]
	astTestMust(t, s.AssignLiteral(a, NewSMTBooleanAxiom()))
	astTestMust(t, s.AssignLiteral(b, smtCoreWalkReason(t, a)))
	// a was already a base fact when it was supplied as an assumption.
	// It is unnecessary for this query and must not be reached via b.
	s.booleanData[a.Var()].SetAssumption(true)
	astTestMust(t, s.PushScope())
	astTestMust(t, s.AssignLiteral(c, NewSMTBooleanAxiom()))
	s.booleanData[c.Var()].SetAssumption(true)
	astTestMust(t, s.AssignLiteral(d, smtCoreWalkReason(t, b, c)))
	r := smtCoreWalkResolver(t, s)
	core, err := r.UnsatCore(smtCoreWalkReason(t, d), SMTNullLiteral)
	astTestMust(t, err)
	expectSMTRawCore(t, core, []SMTLiteral{c, c})
}

func TestSMTUnsatCoreSkipsAssignmentsAboveConflictLevel(t *testing.T) {
	_, s, ls := smtCoreWalkFixture(t, 3)
	a, b, later := ls[0], ls[1], ls[2]
	astTestMust(t, s.PushScope())
	astTestMust(t, s.AssignLiteral(a, NewSMTBooleanAxiom()))
	s.booleanData[a.Var()].SetAssumption(true)
	astTestMust(t, s.AssignLiteral(b, smtCoreWalkReason(t, a)))
	astTestMust(t, s.beginSearchPersistence(s.scopeLevel, SMTParamsState{}))
	defer func() { astTestMust(t, s.endSearchPersistence()) }()
	astTestMust(t, s.PushScope())
	astTestMust(t, s.AssignLiteral(later, NewSMTBooleanAxiom()))
	s.booleanData[later.Var()].SetAssumption(true)
	r := smtCoreWalkResolver(t, s)
	core, err := r.UnsatCore(smtCoreWalkReason(t, b), SMTNullLiteral)
	astTestMust(t, err)
	expectSMTRawCore(t, core, []SMTLiteral{a, a})
	// A conflict above the search level is not a terminal unsat core.
	if _, err = r.UnsatCore(smtCoreWalkReason(t, later), SMTNullLiteral); err == nil {
		t.Fatal("nonterminal conflict accepted as an unsat core")
	}
}

func TestSMTUnsatCoreFailedConsequentPolarityAndContextDedup(t *testing.T) {
	m := astTestManager(t)
	k := smtTestKernel(t, m)
	s := k.internalizer
	p := smtRelevancyBool(t, m, "core-failed-assumption")
	notP, err := m.MkNot(p)
	astTestMust(t, err)
	astTestPin(t, m, notP)
	astTestMust(t, s.Internalize(p, true))
	literal, err := s.Literal(p)
	astTestMust(t, err)
	astTestMust(t, s.PushScope())
	astTestMust(t, s.AssignLiteral(literal, NewSMTBooleanAxiom()))
	s.booleanData[literal.Var()].SetAssumption(true)
	k.assumptions = map[SMTLiteral]ast.ASTHandle{literal.Negated(): notP}
	r := smtCoreWalkResolver(t, s)
	core, err := r.UnsatCore(NewSMTBooleanAxiom(), literal)
	astTestMust(t, err)
	expectSMTRawCore(t, core, []SMTLiteral{literal.Negated(), literal.Negated(), literal})
	astTestMust(t, k.captureUnsatCore(r, NewSMTBooleanAxiom(), literal))
	if got := k.UnsatCore(); len(got) != 1 || got[0] != notP {
		t.Fatal("context did not map and deduplicate raw polarities", got)
	}
	k.assumptions = nil
}

func TestSMTUnsatCoreBooleanReasonVariants(t *testing.T) {
	for _, kind := range []string{"binary", "object", "clause-first", "clause-second", "clause-owner"} {
		t.Run(kind, func(t *testing.T) {
			m, s, ls := smtCoreWalkFixture(t, 3)
			a, b, c := ls[0], ls[1], ls[2]
			astTestMust(t, s.PushScope())
			astTestMust(t, s.AssignLiteral(a, NewSMTBooleanAxiom()))
			s.booleanData[a.Var()].SetAssumption(true)
			astTestMust(t, s.AssignLiteral(b, NewSMTBooleanAxiom()))
			s.booleanData[b.Var()].SetAssumption(true)
			reason := smtCoreWalkReason(t, a)
			switch kind {
			case "binary":
				reason = NewSMTBooleanBinaryClause(a)
			case "clause-first", "clause-second", "clause-owner":
				literals := []SMTLiteral{c, a.Negated()}
				if kind == "clause-second" {
					literals[0], literals[1] = literals[1], literals[0]
				}
				var owner SMTJustification
				if kind == "clause-owner" {
					owner, _ = NewSMTSimpleJustification(true, []SMTLiteral{b})
				}
				clause, err := NewSMTClause(m, literals, SMTClauseAux, owner, nil, false, nil)
				astTestMust(t, err)
				t.Cleanup(func() { astTestMust(t, clause.Close()) })
				reason = NewSMTBooleanClause(clause)
			}
			astTestMust(t, s.AssignLiteral(c, reason))
			r := smtCoreWalkResolver(t, s)
			core, err := r.UnsatCore(smtCoreWalkReason(t, c), SMTNullLiteral)
			astTestMust(t, err)
			want := []SMTLiteral{a, a}
			if kind == "clause-owner" {
				want = []SMTLiteral{a, b, b, a}
			}
			expectSMTRawCore(t, core, want)
		})
	}
}

type smtCoreWalkStoppedJustification struct {
	SMTJustificationBase
	stop    bool
	literal SMTLiteral
}

func (j *smtCoreWalkStoppedJustification) MarkAntecedents(marker SMTAntecedentMarker) error {
	if j.stop {
		return ast.ErrASTDomain
	}
	return marker.MarkLiteral(j.literal)
}

func TestSMTUnsatCoreErrorCleanupAndEmptyTrail(t *testing.T) {
	_, s, ls := smtCoreWalkFixture(t, 2)
	a, b := ls[0], ls[1]
	r := smtCoreWalkResolver(t, s)
	core, err := r.UnsatCore(NewSMTBooleanAxiom(), SMTNullLiteral)
	astTestMust(t, err)
	expectSMTRawCore(t, core, nil)
	astTestMust(t, s.PushScope())
	astTestMust(t, s.AssignLiteral(a, NewSMTBooleanAxiom()))
	s.booleanData[a.Var()].SetAssumption(true)
	owner := &smtCoreWalkStoppedJustification{SMTJustificationBase: NewSMTJustificationBase(true), stop: true, literal: a}
	reason, err := NewSMTBooleanJustification(owner)
	astTestMust(t, err)
	astTestMust(t, s.AssignLiteral(b, reason))
	conflict := smtCoreWalkReason(t, b)
	if _, err = r.UnsatCore(conflict, SMTNullLiteral); err == nil || owner.IsMarked() {
		t.Fatal("stopped expansion lost its error or retained marks")
	}
	owner.stop = false
	core, err = r.UnsatCore(conflict, SMTNullLiteral)
	astTestMust(t, err)
	expectSMTRawCore(t, core, []SMTLiteral{a, a})
	if owner.IsMarked() {
		t.Fatal("successful expansion retained marks")
	}
	// The source reserved variable zero never names an assumption,
	// regardless of the constant literal's polarity.
	core, err = r.UnsatCore(smtCoreWalkReason(t, SMTTrueLiteral, SMTFalseLiteral, a), SMTNullLiteral)
	astTestMust(t, err)
	expectSMTRawCore(t, core, []SMTLiteral{a, a})
}
