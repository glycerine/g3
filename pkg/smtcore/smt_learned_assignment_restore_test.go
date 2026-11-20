package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTLearnedClauseRestorePropagatesAcrossFalseWatchOnce(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	var expressions []ast.ASTHandle
	var literals []SMTLiteral
	for _, name := range []string{"restore-forced-head", "restore-false-watch", "restore-consequence"} {
		expression := smtRelevancyBool(t, m, name)
		astTestMust(t, state.Internalize(expression, true))
		literal, err := state.Literal(expression)
		astTestMust(t, err)
		expressions = append(expressions, expression)
		literals = append(literals, literal)
	}
	// The caller's scope fixes B=false and keeps A=>C. Restoring the learned
	// A or B clause must publish A once and make C propagate from that event.
	astTestMust(t, state.AssignLiteral(literals[1].Negated(), NewSMTBooleanAxiom()))
	astTestMust(t, state.AddLearnedClause([]SMTLiteral{literals[0].Negated(), literals[2]}))
	baseClauses := len(state.testClauses())
	smtTestUserPush(t, state)
	astTestMust(t, state.AddLearnedClause([]SMTLiteral{literals[0], literals[1]}))
	astTestMust(t, state.testClauses()[len(state.testClauses())-1].clause.SetActivity(19))
	saved, err := state.preserveLearnedClauses(baseClauses)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, state.releasePreservedLearnedClauses(saved)) })
	astTestMust(t, state.PopScopes(1))
	before := len(state.assignedLiterals)
	value, err := state.Assignment(literals[0])
	astTestMust(t, err)
	if value != lbool.Undef {
		t.Fatal("scoped head assignment survived the pop", value)
	}
	astTestMust(t, state.restoreLearnedClauses(saved))
	astTestMust(t, state.restoreLearnedClauses(saved))
	if len(state.testClauses()) != baseClauses+1 || len(state.assignedLiterals) != before+1 || state.assignedLiterals[before] != literals[0] {
		t.Fatal("restoration lost or duplicated clause assignment", len(state.testClauses()), len(state.assignedLiterals))
	}
	data := state.booleanData[literals[0].Var()]
	if data.ScopeLevel() != 0 || state.context.assignments.get(expressions[0]) != lbool.True || !data.Justification().Equal(NewSMTBooleanClause(saved[0].clause)) {
		t.Fatal("restored assignment lost its context or clause justification")
	}
	activity, err := saved[0].clause.Activity()
	astTestMust(t, err)
	if activity != 19 {
		t.Fatal("restoration lost learned activity", activity)
	}
	_, err = state.Propagate()
	astTestMust(t, err)
	value, err = state.Assignment(literals[2])
	astTestMust(t, err)
	if value != lbool.True || state.Inconsistent() {
		t.Fatal("restored head did not propagate its caller consequence", value, state.Inconsistent())
	}
}

func TestSMTLearnedUnitRestorePublishesProofAssignmentOnce(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	astTestMust(t, m.ToggleProofMode(ast.ASTProofEnabled))
	expression := smtRelevancyBool(t, m, "restored-proof-unit")
	astTestMust(t, state.Internalize(expression, true))
	literal, err := state.Literal(expression)
	astTestMust(t, err)
	proof, err := m.MkProofAsserted(expression)
	astTestMust(t, err)
	astTestPin(t, m, proof)
	assigned, owners := len(state.assignedLiterals), len(state.ownedJustifications)
	beforeProof, err := m.GetRefCount(proof)
	astTestMust(t, err)
	astTestMust(t, state.beginSearchPersistence(0, SMTParamsState{}))
	astTestMust(t, state.PushScope())
	astTestMust(t, state.AddLearnedClauseWithProof([]SMTLiteral{literal}, proof))
	if len(state.learnedUnits) != 1 {
		t.Fatal("unit was not captured for backtracking")
	}
	// End search while the unit is still installed above its owning scope.
	// The pop to the base level then reasserts it, as reassert_units does.
	astTestMust(t, state.endSearchPersistence())
	if len(state.learnedUnits) != 1 || state.learnedUnits[0].installedLevel != 1 {
		t.Fatal("search end dropped its persistent unit")
	}
	astTestMust(t, state.PopScopes(1))
	astTestMust(t, state.restoreLearnedUnits(0))
	astTestMust(t, state.restoreLearnedUnits(0))
	// reassert_units resets m_units_to_reassert at the base level
	// (smtcore_04-F5), so the restored unit leaves the persistent list.
	if len(state.assignedLiterals) != assigned+1 || len(state.ownedJustifications) != owners+1 || len(state.learnedUnits) != 0 {
		t.Fatal("unit retry duplicated publication or kept its persistence", len(state.assignedLiterals), len(state.ownedJustifications), state.learnedUnits)
	}
	owner, ok := state.ownedJustifications[owners].(*SMTJustificationProofWrapper)
	if !ok {
		t.Fatal("restored unit did not own its proof justification")
	}
	gotProof, err := owner.Proof()
	astTestMust(t, err)
	justification, err := NewSMTBooleanJustification(owner)
	astTestMust(t, err)
	if gotProof != proof || !state.booleanData[literal.Var()].Justification().Equal(justification) || state.context.assignments.get(expression) != lbool.True {
		t.Fatal("restored unit lost its proof or assignment")
	}
	afterProof, err := m.GetRefCount(proof)
	astTestMust(t, err)
	// Only the restored assignment's owner holds one.
	if afterProof != beforeProof+1 {
		t.Fatal("unit restoration acquired the wrong proof ownership", beforeProof, afterProof)
	}
	astTestMust(t, state.endSearchPersistence())
	if len(state.learnedUnits) != 0 {
		t.Fatal("base-installed unit kept redundant persistence roots")
	}
	gotProof, err = owner.Proof()
	astTestMust(t, err)
	if gotProof != proof {
		t.Fatal("persistence cleanup invalidated the live assignment proof")
	}
	astTestMust(t, state.AssignLiteral(literal.Negated(), NewSMTBooleanAxiom()))
	if !state.Inconsistent() {
		t.Fatal("restored unit did not constrain subsequent assignments")
	}
}

func TestSMTRestoredTernaryWatchOrder(t *testing.T) {
	var s SMTInternalizerState
	s.assignment = make([]lbool.LBool, 6)
	s.booleanData = make([]SMTBoolVarData, 3)
	lits := [3]SMTLiteral{NewSMTLiteral(0, false), NewSMTLiteral(1, true), NewSMTLiteral(2, false)}
	values := [...]lbool.LBool{lbool.Undef, lbool.True, lbool.False}
	for _, a := range values {
		for _, b := range values {
			for _, c := range values {
				for i, v := range [...]lbool.LBool{a, b, c} {
					s.assignment[lits[i].Index()] = v
					s.booleanData[i].SetScopeLevel(uint32(2 - i))
				}
				want, got := lits, lits
				for i := 0; i < 2; i++ {
					j, err := s.selectWatchLiteral(want[:], i)
					if err != nil {
						t.Fatal(err)
					}
					want[i], want[j] = want[j], want[i]
				}
				if err := s.selectRestoredWatches(got[:]); err != nil || got != want {
					t.Fatalf("assignments %v %v %v produced %v, want %v, error %v", a, b, c, got, want, err)
				}
			}
		}
	}
}
