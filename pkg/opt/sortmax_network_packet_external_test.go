package opt_test

// Source: src/opt/sortmax.cpp fresh comparator network and model filtering.

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/smtcore"
)

func TestSortMaxPacketUsesDirectCardinalityNetworkAndFiltersWires(t *testing.T) {
	manager, evaluator := priorityFixture(t)
	key := "maxsat_engine"
	name, err := manager.Symbols.Intern("sortmax")
	if err != nil {
		t.Fatal(err)
	}
	if err = evaluator.Context.SetSymbolName(evaluator.Params, &key, name); err != nil {
		t.Fatal(err)
	}
	variables := []ast.ASTHandle{priorityBool(t, manager, "packet-wire-a"), priorityBool(t, manager, "packet-wire-b"), priorityBool(t, manager, "packet-wire-c")}
	solver, err := opt.NewArithmeticSolver(manager, evaluator.Params, "", smtcore.SMTSolverConfig{Params: evaluator.Context, Globals: evaluator.Globals, ModelEvaluator: evaluator})
	if err != nil {
		t.Fatal(err)
	}
	defer solver.Close()
	if err = solver.AddSoftObjective(variables, []*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1), big.NewRat(1, 1)}); err != nil {
		t.Fatal(err)
	}
	result, err := solver.Check()
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	statistics := result.MaxSMT[0]
	if result.Values[0].SoftCost.Cmp(big.NewRat(0, 1)) != 0 || result.Values[0].SoftSatisfied.Cmp(big.NewRat(3, 1)) != 0 {
		t.Fatal("source LE network objective bounds", result.Values[0].SoftCost.RatString(), result.Values[0].SoftSatisfied.RatString())
	}
	if statistics.Inputs != 3 || statistics.Auxiliaries != 3 || statistics.NetworkClauses != 7 || !result.ModelFiltered {
		t.Fatal("SortMax network", statistics, result.ModelFiltered)
	}
	modelEvaluator, err := model.NewModelEvaluator(result.Model, evaluator)
	if err != nil {
		t.Fatal(err)
	}
	defer modelEvaluator.Close()
	if err = modelEvaluator.SetModelCompletion(true); err != nil {
		t.Fatal(err)
	}
	for _, variable := range variables {
		value, evalErr := modelEvaluator.EvaluateValue(variable)
		if evalErr != nil {
			t.Fatal(evalErr)
		}
		h, evalErr := manager.RefGet(value)
		isTrue := evalErr == nil && manager.IsTrue(h)
		evalErr = errors.Join(evalErr, manager.DelRef(&value))
		if evalErr != nil {
			t.Fatal(evalErr)
		}
		if isTrue {
			t.Fatal("source LE network leaves unneeded soft inputs false")
		}
	}
}
