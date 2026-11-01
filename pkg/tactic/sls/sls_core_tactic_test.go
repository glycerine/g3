package sls

// Source: src/tactic/sls/sls_tactic.cpp and src/tactic/sls/sls_engine.cpp.

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func TestSLSCoreCleanupPreservesEngineOnConstructionFailure(t *testing.T) {
	_, manager, _, _, context, globals, local, budget := slsTrackerFixture(t)
	core, err := NewCoreTactic(manager, context, globals, local)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		budget.FailAfter(-1)
		if err := core.Close(); err != nil {
			t.Error(err)
		}
	})
	original := core.owner
	budget.FailAfter(0)
	err = core.Cleanup()
	budget.FailAfter(-1)
	if !errors.Is(err, arena.ErrInjectedFailure) {
		t.Fatal("expected replacement allocation failure", err)
	}
	if core.owner != original || core.open() != nil {
		t.Fatal("failed cleanup discarded the active engine")
	}
	atom := slsBoolConst(t, manager, "sls-cleanup-recovery")
	slsPin(t, manager, atom)
	goal, err := tactic.NewGoal(manager, false, true, false)
	if err != nil {
		t.Fatal(err)
	}
	defer goal.Close()
	if err = goal.Assert(atom); err != nil {
		t.Fatal(err)
	}
	var result tactic.TacticResult
	defer result.Close()
	if err = core.Apply(goal, &result); err != nil {
		t.Fatal("engine unusable after failed cleanup", err)
	}
	if size, err := goal.Size(); err != nil || size != 0 || result.ModelConverter == nil {
		t.Fatal("engine did not solve after failed cleanup", size, err)
	}
	if err = core.Cleanup(); err != nil {
		t.Fatal("cleanup retry", err)
	}
	if core.owner == original || !original.closed {
		t.Fatal("successful cleanup did not replace the engine")
	}
}

func TestSLSCoreCleanupAllocationFrontiers(t *testing.T) {
	succeeded, failures := false, 0
	for stop := int64(0); stop < 512 && !succeeded; stop++ {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			_, manager, _, _, context, globals, local, budget := slsTrackerFixture(t)
			core, err := NewCoreTactic(manager, context, globals, local)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				budget.FailAfter(-1)
				if err := core.Close(); err != nil {
					t.Error(err)
				}
			})
			original := core.owner
			original.engine.stats.Moves = 37
			budget.FailAfter(stop)
			err = core.Cleanup()
			budget.FailAfter(-1)
			if err == nil {
				succeeded = true
				if core.owner == original || !original.closed || core.owner.engine.stats.Moves != 0 {
					t.Fatal("successful cleanup did not reset the engine")
				}
				return
			}
			var construction memory.ConstructionFailure
			if errors.As(err, &construction) {
				if err := construction.Cleanup(); err != nil {
					t.Fatal(err)
				}
			}
			var memout memory.Failure
			if !errors.Is(err, arena.ErrInjectedFailure) && (!errors.As(err, &memout) || memout.Kind != memory.FailureMemout) {
				t.Fatal("unexpected cleanup failure", err)
			}
			if core.owner != original || core.open() != nil || core.owner.engine.stats.Moves != 37 {
				t.Fatal("failed replacement changed the running engine")
			}
			if err = core.Cleanup(); err != nil {
				t.Fatal("cleanup retry", err)
			}
			failures++
		})
		if t.Failed() {
			t.Fatal("cleanup allocation frontier", stop)
		}
	}
	if !succeeded || failures == 0 {
		t.Fatal("did not reach both failure and successful replacement", failures, succeeded)
	}
}

func TestSLSCoreTacticAppliesCleansAndReusesFreshEngine(t *testing.T) {
	_, manager, _, _, context, globals, local, _ := slsTrackerFixture(t)
	core, err := NewCoreTactic(manager, context, globals, local)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := core.Close(); err != nil {
			t.Error(err)
		}
	})
	apply := func(name string) {
		atom := slsBoolConst(t, manager, name)
		slsPin(t, manager, atom)
		goal, err := tactic.NewGoal(manager, false, true, false)
		if err != nil {
			t.Fatal(err)
		}
		defer goal.Close()
		if err = goal.Assert(atom); err != nil {
			t.Fatal(err)
		}
		result := &tactic.TacticResult{}
		defer result.Close()
		if err = core.Apply(goal, result); err != nil {
			t.Fatal(err)
		}
		if size, err := goal.Size(); err != nil || size != 0 {
			t.Fatal("SAT goal was not reset", size, err)
		}
		if result.ModelConverter == nil || len(result.Goals) != 1 {
			t.Fatal("source goal/model result", result.ModelConverter, len(result.Goals))
		}
	}
	apply("sls-core-first")
	stats := core.CollectStatistics()
	if stats["sls moves"] == 0 || stats["sls incr evals"] == 0 {
		t.Fatal("source statistics missing", stats)
	}
	if err = core.Cleanup(); err != nil {
		t.Fatal(err)
	}
	apply("sls-core-second")
}

func TestSLSCoreCancellationUsesSourceException(t *testing.T) {
	_, manager, _, _, context, globals, local, _ := slsTrackerFixture(t)
	core, err := NewCoreTactic(manager, context, globals, local)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := core.Close(); err != nil {
			t.Error(err)
		}
	}()
	atom := slsBoolConst(t, manager, "sls-canceled")
	slsPin(t, manager, atom)
	goal, err := tactic.NewGoal(manager, false, true, false)
	if err != nil {
		t.Fatal(err)
	}
	defer goal.Close()
	if err = goal.Assert(atom); err != nil {
		t.Fatal(err)
	}
	var result tactic.TacticResult
	defer result.Close()
	callbacks := 0
	core.owner.engine.checkpoint = func() error { callbacks++; return nil }
	manager.ResourceLimit().IncCancel()
	reason := manager.ResourceLimit().CancelMessage()
	err = core.Apply(goal, &result)
	manager.ResourceLimit().DecCancel()
	var canceled tactic.TacticException
	if !errors.As(err, &canceled) || canceled.Message() != reason {
		t.Fatal("source cancellation exception", err, reason)
	}
	if callbacks != 0 {
		t.Fatal("cooperation callback ran after cancellation", callbacks)
	}
	depth, err := goal.Depth()
	if err != nil || depth != 0 {
		t.Fatal("canceled goal depth", depth, err)
	}
	if err = core.Cleanup(); err != nil {
		t.Fatal("cleanup after cancellation", err)
	}
	var retry tactic.TacticResult
	defer retry.Close()
	if err = core.Apply(goal, &retry); err != nil {
		t.Fatal("retry after cancellation", err)
	}
	if size, err := goal.Size(); err != nil || size != 0 || retry.ModelConverter == nil {
		t.Fatal("retry did not solve the goal", size, err)
	}
}

func TestSLSCoreTacticParamsStatisticsAndReferenceLifecycle(t *testing.T) {
	_, manager, _, _, context, globals, local, budget := slsTrackerFixture(t)
	core, err := NewCoreTactic(manager, context, globals, local)
	if err != nil {
		t.Fatal(err)
	}
	descriptions, err := uparams.NewParamDescrs(manager.Symbols, uparams.ParamDescrsConfig{Budget: budget})
	if err != nil {
		t.Fatal(err)
	}
	defer descriptions.Close()
	if err = core.CollectParamDescrs(descriptions); err != nil {
		t.Fatal(err)
	}
	if size, err := descriptions.Size(); err != nil || size < 10 {
		t.Fatal("generated SLS parameter descriptions", size, err)
	}
	if err = core.ResetStatistics(); err != nil {
		t.Fatal(err)
	}
	if core.CollectStatistics()["sls moves"] != 0 {
		t.Fatal("statistics reset")
	}
	if err = core.IncRef(); err != nil {
		t.Fatal(err)
	}
	if err = core.DecRef(); err != nil {
		t.Fatal(err)
	}
}

func TestSLSCoreParameterUpdateKeepsTrackerSnapshotUntilCleanup(t *testing.T) {
	_, manager, _, _, context, globals, local, _ := slsTrackerFixture(t)
	core, err := NewCoreTactic(manager, context, globals, local)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := core.Close(); err != nil {
			t.Error(err)
		}
	}()
	if !core.owner.tracker.ucb {
		t.Fatal("source tracker default should enable UCB")
	}
	updated, err := context.NewValue()
	if err != nil {
		t.Fatal(err)
	}
	defer context.Del(&updated)
	ucb, restart := "walksat_ucb", "restart_base"
	if err = context.SetBoolName(&updated, &ucb, false); err != nil {
		t.Fatal(err)
	}
	if err = context.SetUIntName(&updated, &restart, 17); err != nil {
		t.Fatal(err)
	}
	if err = core.UpdateParams(&updated); err != nil {
		t.Fatal(err)
	}
	if !core.owner.tracker.ucb || core.owner.engine.restartBase != 17 {
		t.Fatal("source engine update changed the tracker snapshot", core.owner.tracker.ucb, core.owner.engine.restartBase)
	}
	if err = core.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if core.owner.tracker.ucb || core.owner.engine.restartBase != 17 {
		t.Fatal("fresh engine did not use the updated parameters", core.owner.tracker.ucb, core.owner.engine.restartBase)
	}
}
