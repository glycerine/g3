package smtcore

import "testing"

func TestSMTClauseMaintenanceDeletesInactiveLemmasAndRepairsScopes(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	config := defaultSMTClauseMaintenanceConfig()
	config.Half = true
	config.Recent = 1
	config.Initial = 0
	astTestMust(t, state.ConfigureClauseMaintenance(config))
	literals := make([]SMTLiteral, 8)
	for index := range literals {
		expression := smtRelevancyBool(t, m, "clause-gc-"+string(rune('a'+index)))
		astTestMust(t, state.Internalize(expression, true))
		variable, found := state.BoolVar(expression)
		if !found {
			t.Fatal("missing clause-GC variable", index)
		}
		literals[index] = NewSMTLiteral(variable, false)
	}
	smtTestUserPush(t, state)
	for index := 0; index < 3; index++ {
		astTestMust(t, state.AddLearnedClause(literals[index:index+3]))
	}
	// A search scope above the base level: del_inactive_lemmas1 starts at
	// the base scope's m_lemmas_lim, so all six lemmas are candidates.
	astTestMust(t, state.PushScope())
	for index := 3; index < 6; index++ {
		astTestMust(t, state.AddLearnedClause(literals[index:index+3]))
	}
	astTestMust(t, state.garbageCollectLearnedClauses())
	if len(state.clauses) != 3 || state.statistics.NumDelClause != 3 {
		t.Fatal("learned-clause collection", len(state.clauses), state.statistics.NumDelClause)
	}
	if limit := state.scopes[1].clauseLimit; limit != 2 {
		t.Fatal("repaired inner clause limit", limit)
	}
	astTestMust(t, state.CheckInvariant())
	astTestMust(t, state.PopScopes(1))
	if len(state.clauses) != 3 {
		t.Fatal("lemmas kept across a search pop", len(state.clauses))
	}
	astTestMust(t, state.CheckInvariant())
	astTestMust(t, state.PopScopes(1))
	if len(state.clauses) != 0 {
		t.Fatal("repaired outer clause limit", len(state.clauses))
	}
}
