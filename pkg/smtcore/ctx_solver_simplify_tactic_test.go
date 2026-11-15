package smtcore

import (
	"github.com/glycerine/g3/pkg/util/statistics"
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
	// The source never increments m_num_steps, and statistics::update
	// drops a zero count.
	stats := statistics.Sums(unit.CollectStatistics)
	if _, ok := stats["solver-simplify-steps"]; ok {
		t.Fatal("steps", stats)
	}
	astTestMust(t, unit.Cleanup())
	if statistics.Sums(unit.CollectStatistics)["solver-simplify-steps"] != 0 {
		t.Fatal("cleanup did not reset steps")
	}
	astTestMust(t, unit.ResetStatistics())
}

// smtcore_01-F5: the tactic's smt::kernel is built over a default smt_params
// and updt_params only appends to the context params_ref, so a tactic-level
// max_conflicts 0 does not stop the inner checks.
func TestContextSolverSimplifyIgnoresTacticSMTParams(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	c := config.Context
	settings, err := c.NewValue()
	astTestMust(t, err)
	t.Cleanup(func() { astTestMust(t, c.Del(&settings)) })
	key := "max_conflicts"
	astTestMust(t, c.SetUIntName(&settings, &key, 0))
	tacticConfig := SMTSolverConfig{Params: c, Globals: config.Globals, ModelEvaluator: config}
	for _, viaConstructor := range []bool{false, true} {
		var unit *ContextSolverSimplifyTactic
		if viaConstructor {
			unit, err = NewContextSolverSimplifyTactic(m, &settings, tacticConfig)
		} else {
			unit, err = NewContextSolverSimplifyTactic(m, nil, tacticConfig)
		}
		astTestMust(t, err)
		if !viaConstructor {
			astTestMust(t, unit.UpdateParams(&settings))
		}
		if unit.solver.kernel.smtState.MaxConflicts != ^uint32(0) {
			t.Fatal("tactic params reached the inner smt_params", viaConstructor)
		}
		p := goalTestAtom(t, m, "ctx-solver-p")
		q := goalTestAtom(t, m, "ctx-solver-q")
		disjunction, err := m.MkOr(p, q)
		astTestMust(t, err)
		astTestPin(t, m, disjunction)
		goal, err := tactic.NewGoal(m, false, false, false)
		astTestMust(t, err)
		astTestMust(t, goal.Assert(p))
		astTestMust(t, goal.Assert(disjunction))
		var result tactic.TacticResult
		astTestMust(t, unit.Apply(goal, &result))
		count, err := goal.Size()
		astTestMust(t, err)
		if count != 1 {
			t.Fatalf("output formulas = %d, want 1 (constructor %v)", count, viaConstructor)
		}
		astTestMust(t, unit.Cleanup())
		if unit.solver.kernel.smtState.MaxConflicts != ^uint32(0) {
			t.Fatal("cleanup rebuilt the inner solver from tactic params")
		}
		astTestMust(t, result.Close())
		astTestMust(t, goal.Close())
		astTestMust(t, unit.Close())
	}
}
