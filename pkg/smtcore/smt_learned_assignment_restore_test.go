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
	baseClauses := len(state.clauses)
	astTestMust(t, state.PushScope())
	astTestMust(t, state.AddLearnedClause([]SMTLiteral{literals[0], literals[1]}))
	astTestMust(t, state.clauses[len(state.clauses)-1].clause.SetActivity(19))
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
	if len(state.clauses) != baseClauses+1 || len(state.assignedLiterals) != before+1 || state.assignedLiterals[before] != literals[0] {
		t.Fatal("restoration lost or duplicated clause assignment", len(state.clauses), len(state.assignedLiterals))
	}
	data := state.booleanData[literals[0].Var()]
	if data.ScopeLevel != 0 || state.context.assignments.get(expressions[0]) != lbool.True || data.Justification() != NewSMTBooleanClause(saved[0].clause) {
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
	astTestMust(t, state.beginSearchPersistence(0, SMTParamsState{}))
	astTestMust(t, state.PushScope())
	astTestMust(t, state.AddLearnedClauseWithProof([]SMTLiteral{literal}, proof))
	if len(state.learnedUnits) != 1 {
		t.Fatal("unit was not captured for backtracking")
	}
	// End search while the unit is still installed above its owning scope,
	// then retire that assignment before explicitly restoring the saved unit.
	astTestMust(t, state.endSearchPersistence())
	astTestMust(t, state.PopScopes(1))
	if len(state.learnedUnits) != 1 || state.learnedUnits[0].installedLevel != 1 {
		t.Fatal("scope retirement dropped its persistent unit")
	}
	assigned, owners := len(state.assignedLiterals), len(state.ownedJustifications)
	beforeProof, err := m.GetRefCount(proof)
	astTestMust(t, err)
	astTestMust(t, state.restoreLearnedUnits(0))
	astTestMust(t, state.restoreLearnedUnits(0))
	if len(state.assignedLiterals) != assigned+1 || len(state.ownedJustifications) != owners+1 || state.learnedUnits[0].installedLevel != 0 || state.learnedUnits[0].pending.stage != smtLearnedAssignmentDone {
		t.Fatal("unit retry duplicated publication or left its continuation", len(state.assignedLiterals), len(state.ownedJustifications), state.learnedUnits[0])
	}
	owner, ok := state.ownedJustifications[owners].(*SMTJustificationProofWrapper)
	if !ok {
		t.Fatal("restored unit did not own its proof justification")
	}
	gotProof, err := owner.Proof()
	astTestMust(t, err)
	justification, err := NewSMTBooleanJustification(owner)
	astTestMust(t, err)
	if gotProof != proof || state.booleanData[literal.Var()].Justification() != justification || state.context.assignments.get(expression) != lbool.True {
		t.Fatal("restored unit lost its proof or assignment")
	}
	afterProof, err := m.GetRefCount(proof)
	astTestMust(t, err)
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
