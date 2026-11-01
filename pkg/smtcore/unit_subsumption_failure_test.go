package smtcore

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/memory"
)

func TestUnitSubsumptionConstructorAllocationFrontiers(t *testing.T) {
	succeeded, failures := false, 0
	for stop := int64(0); stop < 512 && !succeeded; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			m, config := smtModelGeneratorFixture(t)
			budget := m.Budget()
			budget.FailAfter(stop)
			unit, err := NewUnitSubsumptionTactic(m, nil, SMTSolverConfig{Params: config.Context, Globals: config.Globals, ModelEvaluator: config})
			budget.FailAfter(-1)
			if err == nil {
				succeeded = true
				astTestMust(t, unit.Close())
				return
			}
			var construction memory.ConstructionFailure
			if errors.As(err, &construction) {
				astTestMust(t, construction.Cleanup())
			}
			var memout memory.Failure
			if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &memout) || memout.Kind != memory.FailureMemout) {
				t.Fatal("unexpected construction failure", err)
			}
			failures++
		})
		if t.Failed() {
			t.Fatal("construction allocation frontier", stop)
		}
	}
	if !succeeded || failures == 0 {
		t.Fatal("did not reach both failure and successful construction", failures, succeeded)
	}
}

func TestUnitSubsumptionApplyAllocationFrontiers(t *testing.T) {
	succeeded, failures := false, 0
	for stop := int64(0); stop < 1024 && !succeeded; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			m, config := smtModelGeneratorFixture(t)
			unit, err := NewUnitSubsumptionTactic(m, nil, SMTSolverConfig{Params: config.Context, Globals: config.Globals, ModelEvaluator: config})
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, unit.Close()) })
			goal, err := tactic.NewGoal(m, false, false, false)
			astTestMust(t, err)
			t.Cleanup(func() { astTestMust(t, goal.Close()) })
			a := goalTestAtom(t, m, "subsumption-failure-a")
			b := goalTestAtom(t, m, "subsumption-failure-b")
			weak, err := m.MkOr(a, b)
			astTestMust(t, err)
			astTestMust(t, goal.Assert(weak))
			astTestMust(t, goal.Assert(a))
			var result tactic.TacticResult
			t.Cleanup(func() { astTestMust(t, result.Close()) })
			base, err := unit.solver.ScopeLevel()
			astTestMust(t, err)
			m.Budget().FailAfter(stop)
			err = unit.Apply(goal, &result)
			m.Budget().FailAfter(-1)
			if err == nil {
				succeeded = true
				return
			}
			var construction memory.ConstructionFailure
			if errors.As(err, &construction) {
				astTestMust(t, construction.Cleanup())
			}
			var memout memory.Failure
			if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &memout) || memout.Kind != memory.FailureMemout) {
				t.Fatal("unexpected apply failure", err)
			}
			var retry tactic.TacticResult
			defer func() { astTestMust(t, retry.Close()) }()
			astTestMust(t, unit.Apply(goal, &retry))
			count, goalErr := goal.Size()
			astTestMust(t, goalErr)
			if count != 1 {
				t.Fatalf("retry produced %d formulas, want 1", count)
			}
			form, goalErr := goal.Form(0)
			astTestMust(t, goalErr)
			if form != a {
				t.Fatal("retry changed the surviving assertion", form, a)
			}
			level, scopeErr := unit.solver.ScopeLevel()
			astTestMust(t, scopeErr)
			if level != base {
				t.Fatalf("retry left solver scope %d, want %d", level, base)
			}
			failures++
		})
		if t.Failed() {
			t.Fatal("apply allocation frontier", stop)
		}
	}
	if !succeeded || failures == 0 {
		t.Fatal("did not reach both failure and successful application", failures, succeeded)
	}
}

func TestUnitSubsumptionRetryPreservesConstructorParameters(t *testing.T) {
	m, config := smtModelGeneratorFixture(t)
	c := config.Context
	initial, err := c.NewValue()
	astTestMust(t, err)
	defer func() { astTestMust(t, c.Del(&initial)) }()
	key := "random_seed"
	astTestMust(t, c.SetUIntName(&initial, &key, 17))
	unit, err := NewUnitSubsumptionTactic(m, &initial, SMTSolverConfig{Params: c, Globals: config.Globals, ModelEvaluator: config})
	astTestMust(t, err)
	defer func() { astTestMust(t, unit.Close()) }()
	updated, err := c.NewValue()
	astTestMust(t, err)
	defer func() { astTestMust(t, c.Del(&updated)) }()
	astTestMust(t, c.SetUIntName(&updated, &key, 23))
	astTestMust(t, unit.UpdateParams(&updated))
	goal, err := tactic.NewGoal(m, false, false, false)
	astTestMust(t, err)
	defer func() { astTestMust(t, goal.Close()) }()
	astTestMust(t, goal.Assert(goalTestAtom(t, m, "subsumption-retry")))
	var first, retry tactic.TacticResult
	defer func() { astTestMust(t, first.Close()); astTestMust(t, retry.Close()) }()
	m.Budget().FailAfter(0)
	err = unit.Apply(goal, &first)
	m.Budget().FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("expected allocation failure", err)
	}
	m.Budget().FailAfter(0)
	err = unit.Apply(goal, &retry)
	m.Budget().FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("expected interrupted solver reconstruction", err)
	}
	astTestMust(t, unit.Apply(goal, &retry))
	snapshot, err := c.GetUIntName(&unit.params, &key, 0)
	astTestMust(t, err)
	solverValue, err := c.GetUIntName(&unit.solver.params, &key, 0)
	astTestMust(t, err)
	if snapshot != 23 || solverValue != 17 {
		t.Fatalf("tactic/solver parameters = %d/%d, want 23/17", snapshot, solverValue)
	}
}
