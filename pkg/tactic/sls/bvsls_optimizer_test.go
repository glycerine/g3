package sls

// Source: src/tactic/sls/bvsls_opt_engine.cpp.

import (
	"testing"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestBVSLSOptimizerPreservesHardConstraintAndSavesBestModel(t *testing.T) {
	engine, _, _, _, manager, _ := slsEngineFixture(t)
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
	if err != nil || result.Status != lbool.True || result.Optimum != five {
		t.Fatal("source represents minimization as maximizing bvneg", result, err)
	}
}
