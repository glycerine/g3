package sls

// Source: src/tactic/sls/sls_tactic.cpp:100-153.

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/cooperate"
)

func slsQFBVFactoryFixture(t *testing.T) (QFBVConfig, *ast.ASTManager) {
	t.Helper()
	_, manager, _, _, context, globals, local, budget := slsTrackerFixture(t)
	cooperation, err := cooperate.NewCooperation(budget)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := cooperation.Close(); err != nil {
			t.Error(err)
		}
	})
	config := QFBVConfig{Context: context, Globals: globals, ModelEvaluator: model.ModelEvaluatorConfig{Context: context, Globals: globals, Params: local, Memory: manager.NumeralService().RuntimeOwner(), Cooperation: cooperation}}
	return config, manager
}

func TestQFBVSLSTacticRunsSourcePreambleAndSolver(t *testing.T) {
	config, manager := slsQFBVFactoryFixture(t)
	seed := "random_seed"
	params, err := config.Context.NewValue()
	if err != nil {
		t.Fatal(err)
	}
	defer config.Context.Del(&params)
	if err = config.Context.SetUIntName(&params, &seed, 17); err != nil {
		t.Fatal(err)
	}
	strategy, err := NewQFBVSLSTactic(manager, &params, config)
	if err != nil {
		t.Fatal(err)
	}
	if err = strategy.IncRef(); err != nil {
		t.Fatal(err)
	}
	defer strategy.DecRef()
	if err = manager.RegisterDeclarationPlugins(); err != nil {
		t.Fatal(err)
	}
	sort, err := manager.BitVectorSort(4)
	if err != nil {
		t.Fatal(err)
	}
	x := slsBVConst(t, manager, sort, "qfbv-sls-x")
	three := slsBVNumeral(t, manager, 3, 4)
	formula, err := manager.MkEq(x, three)
	if err != nil {
		t.Fatal(err)
	}
	slsPin(t, manager, x, three, formula)
	goal, err := tactic.NewGoal(manager, false, true, false)
	if err != nil {
		t.Fatal(err)
	}
	defer goal.Close()
	if err = goal.Assert(formula); err != nil {
		t.Fatal(err)
	}
	result := &tactic.TacticResult{}
	defer result.Close()
	if err = strategy.Apply(goal, result); err != nil {
		t.Fatal(err)
	}
	if len(result.Goals) != 1 {
		t.Fatal("preamble/SLS result count", len(result.Goals))
	}
}

func TestQFBVSLSTacticPreambleStillRejectsArithmeticGoal(t *testing.T) {
	config, manager := slsQFBVFactoryFixture(t)
	strategy, err := NewQFBVSLSTactic(manager, nil, config)
	if err != nil {
		t.Fatal(err)
	}
	if err = strategy.IncRef(); err != nil {
		t.Fatal(err)
	}
	defer strategy.DecRef()
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	if err != nil {
		t.Fatal(err)
	}
	sort, err := manager.GetSort(zero)
	if err != nil {
		t.Fatal(err)
	}
	name, err := manager.Symbols.Intern("qfbv-sls-int")
	if err != nil {
		t.Fatal(err)
	}
	x, err := manager.MkConst(name, sort)
	if err != nil {
		t.Fatal(err)
	}
	product, err := manager.MkArithmeticMul(x, x)
	if err != nil {
		t.Fatal(err)
	}
	arithmetic, err := manager.RegisterArithmeticPlugin()
	if err != nil {
		t.Fatal(err)
	}
	formula, err := manager.MkFamilyApp(arithmetic, 2, nil, []ast.ASTHandle{product, zero}) // OP_LE
	if err != nil {
		t.Fatal(err)
	}
	slsPin(t, manager, zero, x, product, formula)
	goal, err := tactic.NewGoal(manager, false, false, false)
	if err != nil {
		t.Fatal(err)
	}
	defer goal.Close()
	if err = goal.Assert(formula); err != nil {
		t.Fatal(err)
	}
	result := &tactic.TacticResult{}
	defer result.Close()
	if err = strategy.Apply(goal, result); err == nil {
		t.Fatal("arithmetic goal survived QF_BV guard")
	}
}
