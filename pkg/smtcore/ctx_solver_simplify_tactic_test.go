package smtcore

import (
	"testing"

	"github.com/glycerine/g3/pkg/tactic"
)

func TestContextSolverSimplifyUsesBooleanContext(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	tacticConfig := SMTSolverConfig{Params: config.Context, Globals: config.Globals, ModelEvaluator: config}
	unit, err := NewContextSolverSimplifyTactic(m, nil, tacticConfig)
	astTestMust(t, err)
	defer func() { astTestMust(t, unit.Close()) }()
	p := goalTestAtom(t, m, "ctx-solver-p")
	q := goalTestAtom(t, m, "ctx-solver-q")
	disjunction, err := m.MkOr(p, q)
	astTestMust(t, err)
	astTestPin(t, m, disjunction)
	goal, err := tactic.NewGoal(m, false, false, false)
	astTestMust(t, err)
	defer func() { astTestMust(t, goal.Close()) }()
	astTestMust(t, goal.Assert(p))
	astTestMust(t, goal.Assert(disjunction))
	var result tactic.TacticResult
	defer func() { astTestMust(t, result.Close()) }()
	astTestMust(t, unit.Apply(goal, &result))
	count, err := goal.Size()
	astTestMust(t, err)
	if count != 1 {
		t.Fatalf("combined output formulas = %d, want 1", count)
	}
	formula, err := goal.Form(0)
	astTestMust(t, err)
	if formula != p {
		t.Fatalf("contextual result = %v, want p (%v)", formula, p)
	}
	stats := unit.CollectStatistics()
	if stats["solver-simplify-steps"] != 1 {
		t.Fatal("steps", stats)
	}
	astTestMust(t, unit.Cleanup())
	if unit.CollectStatistics()["solver-simplify-steps"] != 0 {
		t.Fatal("cleanup did not reset steps")
	}
	astTestMust(t, unit.ResetStatistics())
}
