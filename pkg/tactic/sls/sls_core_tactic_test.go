package sls

// Source: src/tactic/sls/sls_tactic.cpp and src/tactic/sls/sls_engine.cpp.

import (
	"testing"

	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

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
