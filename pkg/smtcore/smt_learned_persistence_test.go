package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestSMTLearnedClauseSurvivesSearchBacktrack(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	first := smtRelevancyBool(t, m, "persistent-clause-first")
	second := smtRelevancyBool(t, m, "persistent-clause-second")

	astTestMust(t, state.beginSearchPersistence(0, SMTParamsState{}))
	defer func() { astTestMust(t, state.endSearchPersistence()) }()
	astTestMust(t, state.PushScope())
	for _, expression := range []ast.ASTHandle{first, second} {
		astTestMust(t, state.Internalize(expression, true))
	}
	firstVar, _ := state.BoolVar(first)
	secondVar, _ := state.BoolVar(second)
	astTestMust(t, state.AddLearnedClauseWithProof([]SMTLiteral{
		NewSMTLiteral(firstVar, false), NewSMTLiteral(secondVar, true),
	}, ast.ASTHandle{}))
	if len(state.clauses) != 1 || state.clauses[0].clause == nil {
		t.Fatal("missing scoped learned clause")
	}
	astTestMust(t, state.clauses[0].clause.SetActivity(9))

	astTestMust(t, state.PopScopes(1))
	if state.scopeLevel != 0 || len(state.clauses) != 1 {
		t.Fatal("learned clause was not moved to the search base", state.scopeLevel, len(state.clauses))
	}
	clause := state.clauses[0].clause
	if clause == nil || !clause.IsLearned() {
		t.Fatal("restored record is not a learned clause")
	}
	activity, err := clause.Activity()
	astTestMust(t, err)
	if activity != 9 {
		t.Fatal("restored learned-clause activity", activity)
	}
	if _, found := state.BoolVar(first); !found {
		t.Fatal("first learned atom was not reinternalized")
	}
	if _, found := state.BoolVar(second); !found {
		t.Fatal("second learned atom was not reinternalized")
	}
}

func TestSMTLearnedUnitSurvivesSearchBacktrackAndExpiresWithBase(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	expression := smtRelevancyBool(t, m, "persistent-unit")

	smtTestUserPush(t, state)
	astTestMust(t, state.beginSearchPersistence(1, SMTParamsState{}))
	astTestMust(t, state.PushScope())
	astTestMust(t, state.Internalize(expression, true))
	variable, found := state.BoolVar(expression)
	if !found {
		t.Fatal("missing scoped unit atom")
	}
	astTestMust(t, state.AddLearnedClauseWithProof([]SMTLiteral{NewSMTLiteral(variable, true)}, ast.ASTHandle{}))
	if len(state.learnedUnits) != 1 {
		t.Fatal("learned unit was not retained", len(state.learnedUnits))
	}

	astTestMust(t, state.PopScopes(1))
	variable, found = state.BoolVar(expression)
	if !found {
		t.Fatal("learned unit atom was not reinternalized")
	}
	value, err := state.Assignment(NewSMTLiteral(variable, true))
	astTestMust(t, err)
	if value != lbool.True {
		t.Fatal("learned unit was not reasserted", value)
	}
	if state.learnedUnits[0].installedLevel != 1 {
		t.Fatal("learned unit installation level", state.learnedUnits[0].installedLevel)
	}

	astTestMust(t, state.endSearchPersistence())
	if len(state.learnedUnits) != 0 {
		t.Fatal("settled learned unit retained search-only ownership")
	}
	astTestMust(t, state.PopScopes(1))
	if len(state.learnedUnits) != 0 {
		t.Fatal("learned unit outlived its user base scope")
	}
	if _, found = state.BoolVar(expression); found {
		t.Fatal("learned unit atom outlived its user base scope")
	}
}
