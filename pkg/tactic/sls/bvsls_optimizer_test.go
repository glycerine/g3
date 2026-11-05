package sls

// Source: src/tactic/sls/bvsls_opt_engine.cpp.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func TestBVSLSOptimizerPreservesHardConstraintAndSavesBestModel(t *testing.T) {
	engine, _, _, _, manager, _ := slsEngineFixture(t)
	engine.maxRestarts = 2
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
	repeated, err := optimizer.Optimize(x, nil, true)
	if err != nil || repeated.Status != lbool.True || !repeated.Optimum.IsNull() {
		t.Fatal("a pass without a new best score must not publish an optimum", repeated, err)
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
		t.Fatal("source minimization comparison does not publish a new optimum against the zero score", result, err)
	}
	family, err := manager.BitVectorUtilityFamilyID()
	if err != nil {
		t.Fatal(err)
	}
	negated, err := manager.MkFamilyApp(family, 3, nil, []ast.ASTHandle{x})
	if err != nil {
		t.Fatal(err)
	}
	equal, err := manager.AreEqual(optimizer.objectiveExpr, negated)
	if err != nil || !equal {
		t.Fatal("source represents minimization by maximizing bvneg", equal, err)
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
		t.Fatal("minimize model", value, err)
	}
}

func TestBVSLSOptimizerUsesOnlySourceEnabledMoves(t *testing.T) {
	engine, _, _, _, manager, numbers := slsEngineFixture(t)
	engine.maxRestarts = 1
	if err := manager.RegisterDeclarationPlugins(); err != nil {
		t.Fatal(err)
	}
	sort, err := manager.BitVectorSort(3)
	if err != nil {
		t.Fatal(err)
	}
	x := slsBVConst(t, manager, sort, "bvsls-source-moves-x")
	one, two := slsBVNumeral(t, manager, 1, 3), slsBVNumeral(t, manager, 2, 3)
	isOne, err := manager.MkEq(x, one)
	if err != nil {
		t.Fatal(err)
	}
	isTwo, err := manager.MkEq(x, two)
	if err != nil {
		t.Fatal(err)
	}
	hard, err := manager.MkOr(isOne, isTwo)
	if err != nil {
		t.Fatal(err)
	}
	slsPin(t, manager, x, one, two, isOne, isTwo, hard)
	if err = engine.AssertExpression(hard); err != nil {
		t.Fatal(err)
	}
	optimizer, err := NewBVOptimizer(engine)
	if err != nil {
		t.Fatal(err)
	}
	defer optimizer.Close()
	if err = engine.search.Initialize(engine.assertions); err != nil {
		t.Fatal(err)
	}
	if err = engine.search.Reset(); err != nil {
		t.Fatal(err)
	}
	decl, err := manager.GetDecl(x)
	if err != nil {
		t.Fatal(err)
	}
	start := numeral.NewInt(1)
	if err = engine.tracker.SetDeclValue(decl, &start); err != nil {
		t.Fatal(err)
	}
	if err = engine.evaluator.UpdateAll(engine.search); err != nil {
		t.Fatal(err)
	}
	if err = optimizer.setupObjective(x, true); err != nil {
		t.Fatal(err)
	}
	if err = optimizer.objective.SetDeclValue(decl, &start); err != nil {
		t.Fatal(err)
	}
	if err = optimizer.evaluator.UpdateAll(optimizer.search); err != nil {
		t.Fatal(err)
	}
	declarations, err := optimizer.objective.Constants()
	if err != nil {
		t.Fatal(err)
	}
	score := numeral.NewInt(1)
	bestValue, maximum := numeral.NewInt(0), numeral.NewInt(7)
	_, _, _, _, found, err := optimizer.findBestMove(declarations, &score, &bestValue, &maximum)
	if err != nil || found {
		t.Fatal("source build enables only bit flips and inversion; +1 to the sole other feasible value is disabled", found, err)
	}
	if err = numbers.Del(&maximum); err != nil {
		t.Fatal(err)
	}
}
