package smtcore

import (
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

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
	if len(state.testClauses()) != 3 || state.statistics.NumDelClause != 3 {
		t.Fatal("learned-clause collection", len(state.testClauses()), state.statistics.NumDelClause)
	}
	// Only base scopes keep an m_lemmas limit; a search pop deletes no
	// lemma, so collection leaves every limit alone.
	if limit := state.scopes[0].lemmaLimit; limit != 0 || len(state.lemmas) != 3 {
		t.Fatal("base-scope lemma limit", limit, len(state.lemmas))
	}
	astTestMust(t, state.CheckInvariant())
	astTestMust(t, state.PopScopes(1))
	if len(state.testClauses()) != 3 {
		t.Fatal("lemmas kept across a search pop", len(state.testClauses()))
	}
	astTestMust(t, state.CheckInvariant())
	astTestMust(t, state.PopScopes(1))
	if len(state.testClauses()) != 0 {
		t.Fatal("base-scope pop kept lemmas", len(state.testClauses()))
	}
}

// AUDIT.md smtcore_06-F4: bounded_search runs del_inactive_lemmas after the
// restart and conflict-limit exits, so a restart at the conflict that
// crosses the GC threshold postpones the collection.
func TestSMTLemmaGCSkippedWhenRestartIsDue(t *testing.T) {
	m := astTestManager(t)
	k := smtTestKernel(t, m)
	const pigeons, holes = 7, 6
	var p [pigeons][holes]ast.ASTHandle
	for i := range pigeons {
		for j := range holes {
			p[i][j] = smtRelevancyBool(t, m, fmt.Sprintf("lemma-gc-restart-p-%d-%d", i, j))
		}
		clause, err := m.MkOrArray(p[i][:])
		astTestMust(t, err)
		astTestMust(t, k.Assert(clause, ast.ASTHandle{}))
	}
	for j := range holes {
		for i := range pigeons {
			for l := i + 1; l < pigeons; l++ {
				ni, err := m.MkNot(p[i][j])
				astTestMust(t, err)
				nl, err := m.MkNot(p[l][j])
				astTestMust(t, err)
				clause, err := m.MkOr(ni, nl)
				astTestMust(t, err)
				astTestMust(t, k.Assert(clause, ast.ASTHandle{}))
			}
		}
	}
	ready, err := k.beginCheck()
	astTestMust(t, err)
	if !ready {
		t.Fatal("check preamble")
	}
	astTestMust(t, k.configureCheck(false))
	astTestMust(t, k.internalizeCheckAssertions())
	k.smtState.RestartAdaptive = false
	k.smtState.RestartStrategy = 3 // fixed
	k.smtState.RestartInitial = 1
	k.internalizer.clauseMaintenance.Strategy = 0
	k.internalizer.clauseMaintenance.Initial = 0
	session, err := k.beginSearch()
	astTestMust(t, err)
	defer func() { astTestMust(t, k.endSearch(session)) }()
	if k.internalizer.lemmaGCThreshold != 0 {
		t.Fatal("lemma GC threshold", k.internalizer.lemmaGCThreshold)
	}
	for range 100 {
		status, restartDue, err := k.boundedSearch(session)
		astTestMust(t, err)
		if !restartDue {
			t.Fatal("search ended without a restart", status)
		}
		// With a zero threshold every resolved conflict that does not
		// restart collects and resets the counter; the restarting one
		// leaves it counted.
		if k.internalizer.conflictsSinceLemmaGC == 0 {
			t.Fatal("lemma GC ran at the restarting conflict")
		}
		if k.internalizer.statistics.NumConflicts > session.conflictBase+1 {
			return
		}
		astTestMust(t, k.restartSearch(session.baseLevel, &session.restarts))
	}
	t.Fatal("no restart after a non-restarting conflict")
}

// AUDIT.md smtcore_02-F10: del_inactive_lemmas1 stable-sorts m_lemmas in
// place, so the next collection breaks activity ties in the previous
// activity order, not in creation order.
func TestSMTClauseMaintenanceHalfKeepsSortedOrder(t *testing.T) {
	m := astTestManager(t)
	_, _, state := smtInternalizerFixture(t, m, nil)
	config := defaultSMTClauseMaintenanceConfig()
	config.Half = true
	config.Recent = 0
	astTestMust(t, state.ConfigureClauseMaintenance(config))
	literals := make([]SMTLiteral, 6)
	for index := range literals {
		expression := smtRelevancyBool(t, m, "half-order-"+string(rune('a'+index)))
		astTestMust(t, state.Internalize(expression, true))
		variable, found := state.BoolVar(expression)
		if !found {
			t.Fatal("missing half-order variable", index)
		}
		literals[index] = NewSMTLiteral(variable, false)
	}
	var lemmas [4]*SMTClause
	for index, activity := range []uint32{1, 2, 2, 1} {
		astTestMust(t, state.AddLearnedClause(literals[index:index+3]))
		lemmas[index] = state.lemmas[len(state.lemmas)-1].clause
		astTestMust(t, lemmas[index].SetActivity(activity))
	}
	order := func() []*SMTClause {
		var result []*SMTClause
		for index, record := range state.lemmas {
			if record.clause.lemmaIndex != index {
				t.Fatalf("lemma at %d still records index %d", index, record.clause.lemmaIndex)
			}
			result = append(result, record.clause)
		}
		return result
	}
	// First collection: every lemma is pinned (can_delete is false for a
	// clause on the reinit stack), so only the sort shows.
	for _, lemma := range lemmas {
		lemma.reinit = true
	}
	astTestMust(t, state.garbageCollectLearnedClauses())
	for _, lemma := range lemmas {
		lemma.reinit = false
	}
	if got, want := order(), []*SMTClause{lemmas[1], lemmas[2], lemmas[0], lemmas[3]}; fmt.Sprint(got) != fmt.Sprint(want) || state.statistics.NumDelClause != 0 {
		t.Fatal("first collection did not leave m_lemmas in activity order", got, want, state.statistics.NumDelClause)
	}
	for _, lemma := range lemmas {
		astTestMust(t, lemma.SetActivity(1))
	}
	astTestMust(t, state.garbageCollectLearnedClauses())
	if got, want := order(), []*SMTClause{lemmas[1], lemmas[2]}; fmt.Sprint(got) != fmt.Sprint(want) || state.statistics.NumDelClause != 2 {
		t.Fatal("second collection broke ties in creation order", got, want, state.statistics.NumDelClause)
	}
}
