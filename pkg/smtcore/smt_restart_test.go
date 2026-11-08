package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func TestSMTRestartSchedulesMatchSource(t *testing.T) {
	tests := []struct {
		name                    string
		strategy                uint32
		factor                  float64
		first, second           uint32
		firstOuter, secondOuter uint32
	}{
		{"geometric", 0, 1.5, 150, 225, 100, 100},
		{"inner-outer", 1, 1.1, 100, 110, 110, 110},
		{"luby", 2, 1.1, 100, 200, 100, 100},
		{"fixed", 3, 1.1, 100, 100, 100, 100},
		{"arithmetic", 4, 1.1, 101, 102, 100, 100},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			params := SMTParamsState{RestartStrategy: test.strategy, RestartInitial: 100, RestartFactor: test.factor}
			state := newSMTRestartState(params)
			state.conflicts = 100
			astTestMust(t, state.advance(params))
			if state.threshold != test.first || state.outerThreshold != test.firstOuter || state.conflicts != 0 {
				t.Fatal("first restart schedule", state)
			}
			state.conflicts = state.threshold
			astTestMust(t, state.advance(params))
			if state.threshold != test.second || state.outerThreshold != test.secondOuter || state.conflicts != 0 {
				t.Fatal("second restart schedule", state)
			}
		})
	}
}

func TestSMTRestartPreservesAssumptionScopeClauseReasons(t *testing.T) {
	m := astTestManager(t)
	k := smtTestKernel(t, m)
	k.smtState.SimplifyClauses = true
	s := k.internalizer
	// A two-literal lemma would be a binary watch (use_binary_clause_opt);
	// this test needs a clause reason.
	s.binaryClauseOptimization = false
	p := smtRelevancyBool(t, m, "restart-assumption-p")
	q := smtRelevancyBool(t, m, "restart-assumption-q")
	for _, h := range []ast.ASTHandle{p, q} {
		astTestMust(t, s.Internalize(h, true))
	}
	pl, err := s.Literal(p)
	astTestMust(t, err)
	ql, err := s.Literal(q)
	astTestMust(t, err)
	astTestMust(t, s.PushScope())
	k.assumptions = map[SMTLiteral]ast.ASTHandle{pl: p}
	defer func() { k.assumptions = nil }()
	astTestMust(t, s.AssignLiteral(pl, NewSMTBooleanAxiom()))
	s.booleanData[pl.Var()].Assumption = true
	astTestMust(t, s.AddLearnedClause([]SMTLiteral{ql, pl.Negated()}))
	_, err = s.Propagate()
	astTestMust(t, err)
	before, found := s.booleanData[ql.Var()].Justification().Clause()
	if !found || before == nil {
		t.Fatal("fixture did not retain a clause reason")
	}
	astTestMust(t, s.beginSearchPersistence(1, k.smtState))
	defer func() { astTestMust(t, s.endSearchPersistence()) }()
	restart := newSMTRestartState(k.smtState)
	astTestMust(t, k.restartSearchAtBase(1, &restart, true))
	after, found := s.booleanData[ql.Var()].Justification().Clause()
	if !found || after != before || len(s.clauses) != 1 {
		t.Fatal("restart simplified above the user base and discarded an assumption reason")
	}
}

func TestSMTRestartBacktracksAndPreservesLearnedClause(t *testing.T) {
	m := astTestManager(t)
	kernel, err := NewSMTKernel(m, nil)
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, kernel.Close()) })
	kernel.smtState.RestartAdaptive = false
	kernel.smtState.RestartInitial = 0
	kernel.smtState.RestartStrategy = 3
	state := kernel.internalizer
	astTestMust(t, state.beginSearchPersistence(0, kernel.smtState))
	defer func() { astTestMust(t, state.endSearchPersistence()) }()
	for range 3 {
		astTestMust(t, state.PushScope())
	}
	first := smtRelevancyBool(t, m, "restart-first")
	second := smtRelevancyBool(t, m, "restart-second")
	for _, expression := range []ast.ASTHandle{first, second} {
		astTestMust(t, state.Internalize(expression, true))
	}
	firstVar, _ := state.BoolVar(first)
	secondVar, _ := state.BoolVar(second)
	astTestMust(t, state.AddLearnedClauseWithProof([]SMTLiteral{
		NewSMTLiteral(firstVar, false), NewSMTLiteral(secondVar, false),
	}, ast.ASTHandle{}))

	restart := newSMTRestartState(kernel.smtState)
	restart.conflicts = 1
	if !restart.due(state.scopeLevel, 0) {
		t.Fatal("restart threshold did not fire")
	}
	astTestMust(t, kernel.restartSearch(0, &restart))
	if state.scopeLevel != 0 || state.statistics.NumRestarts != 1 {
		t.Fatal("restart did not return to search base", state.scopeLevel, state.statistics.NumRestarts)
	}
	if len(state.clauses) != 1 || state.clauses[0].clause == nil || !state.clauses[0].clause.IsLearned() {
		t.Fatal("restart lost learned clause")
	}
}
