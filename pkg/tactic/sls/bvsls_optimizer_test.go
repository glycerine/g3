package sls

// Source: src/tactic/sls/bvsls_opt_engine.cpp.

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestBVSLSOptimizerPreservesHardConstraintAndSavesBestModel(t *testing.T) {
	engine, _, _, _, manager, _ := slsEngineFixture(t)
	engine.maxRestarts = 1
	if err := manager.RegisterDeclarationPlugins(); err != nil {
		t.Fatal(err)
	}
	sort, err := manager.BitVectorSort(3)
	if err != nil {
		t.Fatal(err)
	}
	x := slsBVConst(t, manager, sort, "bvsls-opt-x")
	three := slsBVNumeral(t, manager, 3, 3)
	hard, err := manager.MkEq(x, three)
	if err != nil {
		t.Fatal(err)
	}
	slsPin(t, manager, x, three, hard)
	if err = engine.AssertExpression(hard); err != nil {
		t.Fatal(err)
	}
	optimizer, err := NewBVOptimizer(engine)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := optimizer.Close(); err != nil {
			t.Error(err)
		}
	})
	result, err := optimizer.Optimize(x, nil, true)
	if err != nil || result.Status != lbool.True || result.Optimum != three {
		t.Fatal("maximize fixed feasible value", result, err)
	}
	best, err := optimizer.GetModel()
	if err != nil {
		t.Fatal(err)
	}
	defer best.Close()
	decl, err := manager.GetDecl(x)
	if err != nil {
		t.Fatal(err)
	}
	value, err := best.Core().ConstantInterpretation(decl)
	if err != nil || value != three {
		t.Fatal("saved hard/objective model", value, err)
	}
}

func TestBVSLSOptimizerMinimizeUsesSourceNegatedObjective(t *testing.T) {
	engine, _, _, _, manager, _ := slsEngineFixture(t)
	engine.maxRestarts = 1
	if err := manager.RegisterDeclarationPlugins(); err != nil {
		t.Fatal(err)
	}
	sort, err := manager.BitVectorSort(3)
	if err != nil {
		t.Fatal(err)
	}
	x := slsBVConst(t, manager, sort, "bvsls-min-x")
	three := slsBVNumeral(t, manager, 3, 3)
	five := slsBVNumeral(t, manager, 5, 3)
	hard, err := manager.MkEq(x, three)
	if err != nil {
		t.Fatal(err)
	}
	slsPin(t, manager, x, three, five, hard)
	if err = engine.AssertExpression(hard); err != nil {
		t.Fatal(err)
	}
	optimizer, err := NewBVOptimizer(engine)
	if err != nil {
		t.Fatal(err)
	}
	defer optimizer.Close()
	result, err := optimizer.Optimize(x, nil, false)
	if err != nil || result.Status != lbool.True || !result.Optimum.IsNull() {
		t.Fatal("source minimizes only when its negated score improves below the previous best", result, err)
	}
	if got, e := optimizer.optimumAST(); e != nil || got != five {
		t.Fatal("source best-model score uses bvneg", got, e)
	}
}

func TestBVSLSOptimizerRespectsZeroRestartBudget(t *testing.T) {
	engine, _, _, _, manager, _ := slsEngineFixture(t)
	engine.maxRestarts = 0
	if err := manager.RegisterDeclarationPlugins(); err != nil {
		t.Fatal(err)
	}
	objective := slsBVNumeral(t, manager, 3, 3)
	slsPin(t, manager, objective)
	optimizer, err := NewBVOptimizer(engine)
	if err != nil {
		t.Fatal(err)
	}
	defer optimizer.Close()
	result, err := optimizer.Optimize(objective, nil, true)
	if err != nil || result.Status != lbool.Undef || !result.Optimum.IsNull() || engine.stats.Restarts != 0 {
		t.Fatal("zero restart budget", result, engine.stats.Restarts, err)
	}
}

func TestBVSLSOptimizerRunsConfiguredRestarts(t *testing.T) {
	engine, _, _, _, manager, _ := slsEngineFixture(t)
	engine.maxRestarts = 3
	if err := manager.RegisterDeclarationPlugins(); err != nil {
		t.Fatal(err)
	}
	objective := slsBVNumeral(t, manager, 3, 3)
	slsPin(t, manager, objective)
	optimizer, err := NewBVOptimizer(engine)
	if err != nil {
		t.Fatal(err)
	}
	defer optimizer.Close()
	result, err := optimizer.Optimize(objective, nil, true)
	if err != nil || result.Status != lbool.True || result.Optimum != objective || engine.stats.Restarts != 3 {
		t.Fatal("configured restarts", result, engine.stats.Restarts, err)
	}
}

func TestBVSLSOptimizerCountsUnsuccessfulSatisfactionRestarts(t *testing.T) {
	engine, _, _, _, manager, _ := slsEngineFixture(t)
	engine.maxRestarts, engine.restartBase, engine.restartNext = 2, 1, 0
	if err := manager.RegisterDeclarationPlugins(); err != nil {
		t.Fatal(err)
	}
	sort, err := manager.BitVectorSort(2)
	if err != nil {
		t.Fatal(err)
	}
	x := slsBVConst(t, manager, sort, "bvsls-unsatisfiable-x")
	zero, one := slsBVNumeral(t, manager, 0, 2), slsBVNumeral(t, manager, 1, 2)
	xZero, err := manager.MkEq(x, zero)
	if err != nil {
		t.Fatal(err)
	}
	xOne, err := manager.MkEq(x, one)
	if err != nil {
		t.Fatal(err)
	}
	slsPin(t, manager, x, zero, one, xZero, xOne)
	if err = engine.AssertExpression(xZero); err != nil {
		t.Fatal(err)
	}
	if err = engine.AssertExpression(xOne); err != nil {
		t.Fatal(err)
	}
	optimizer, err := NewBVOptimizer(engine)
	if err != nil {
		t.Fatal(err)
	}
	defer optimizer.Close()
	result, err := optimizer.Optimize(x, nil, true)
	if err != nil || result.Status != lbool.Undef || !result.Optimum.IsNull() || engine.stats.Restarts != 4 {
		t.Fatal("unsuccessful restart accounting", result, engine.stats.Restarts, err)
	}
}
