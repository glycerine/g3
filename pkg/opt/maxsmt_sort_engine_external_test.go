package opt_test

// Source: src/opt/sortmax.cpp end-to-end retained integer-weight engine.

import (
	"errors"
	"math/big"
	"testing"

	_ "github.com/glycerine/g3/pkg/engine"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/opt"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func sortMaxFixture(t *testing.T) (*ast.ASTManager, model.ModelEvaluatorConfig) {
	t.Helper()
	budget := arena.NewBudget(1 << 29)
	symbols, err := usymbol.NewManager(usymbol.Config{Budget: budget})
	if err != nil {
		t.Fatal(err)
	}
	numbers, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	if err != nil {
		t.Fatal(err)
	}
	if err = manager.RegisterDeclarationPlugins(); err != nil {
		t.Fatal(err)
	}
	context, err := uparams.NewParamsContext(symbols, numbers, uparams.ParamsConfig{Budget: budget})
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := uparams.NewGlobalParamsCatalog(symbols, uparams.GlobalParamsCatalogConfig{Budget: budget})
	if err != nil {
		t.Fatal(err)
	}
	globals, err := uparams.NewGlobalParamsService(symbols, context, uparams.GlobalParamsConfig{Budget: budget, Catalog: catalog})
	if err != nil {
		t.Fatal(err)
	}
	if err = globals.InitializeSource(); err != nil {
		t.Fatal(err)
	}
	cooperation, err := cooperate.NewCooperation(budget)
	if err != nil {
		t.Fatal(err)
	}
	params, err := context.NewValue()
	if err != nil {
		t.Fatal(err)
	}
	config := model.ModelEvaluatorConfig{Context: context, Globals: globals, Params: &params, Memory: numbers.RuntimeOwner(), Cooperation: cooperation}
	t.Cleanup(func() {
		if err := context.Del(&params); err != nil {
			t.Error(err)
		}
		if err := cooperation.Close(); err != nil {
			t.Error(err)
		}
		if err := globals.Close(); err != nil {
			t.Error(err)
		}
		if err := catalog.Close(); err != nil {
			t.Error(err)
		}
		if err := context.Close(); err != nil {
			t.Error(err)
		}
		if err := manager.Close(); err != nil {
			t.Error(err)
		}
		if err := numbers.Close(); err != nil {
			t.Error(err)
		}
		if err := symbols.Close(); err != nil {
			t.Error(err)
		}
	})
	return manager, config
}

func sortMaxBool(t *testing.T, manager *ast.ASTManager, name string) ast.ASTHandle {
	t.Helper()
	sort, err := manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	symbol, err := manager.Symbols.Intern(name)
	if err != nil {
		t.Fatal(err)
	}
	expression, err := manager.MkConst(symbol, sort)
	if err != nil {
		t.Fatal(err)
	}
	return expression
}

func TestArithmeticSolverUsesSortMaxForWeightedMutex(t *testing.T) {
	manager, evaluator := sortMaxFixture(t)
	key := "maxsat_engine"
	sortmax, err := manager.Symbols.Intern("sortmax")
	if err != nil {
		t.Fatal(err)
	}
	if err = evaluator.Context.SetSymbolName(evaluator.Params, &key, sortmax); err != nil {
		t.Fatal(err)
	}
	p := sortMaxBool(t, manager, "sortmax-p")
	if err := manager.IncRef(p); err != nil {
		t.Fatal(err)
	}
	defer manager.DecRef(p)
	q := sortMaxBool(t, manager, "sortmax-q")
	if err := manager.IncRef(q); err != nil {
		t.Fatal(err)
	}
	defer manager.DecRef(q)
	r := sortMaxBool(t, manager, "sortmax-r")
	if err := manager.IncRef(r); err != nil {
		t.Fatal(err)
	}
	defer manager.DecRef(r)
	pAndQ, err := manager.MkAnd(p, q)
	if err != nil {
		t.Fatal(err)
	}
	mutex, err := manager.MkNot(pAndQ)
	if err != nil {
		t.Fatal(err)
	}
	for _, expression := range []ast.ASTHandle{pAndQ, mutex} {
		if err = manager.IncRef(expression); err != nil {
			t.Fatal(err)
		}
		defer manager.DecRef(expression)
	}
	config := smtcore.SMTSolverConfig{Params: evaluator.Context, Globals: evaluator.Globals, ModelEvaluator: evaluator}
	solver, err := opt.NewArithmeticSolver(manager, evaluator.Params, "", config)
	if err != nil {
		t.Fatal(err)
	}
	defer solver.Close()
	if err = solver.Assert(mutex); err != nil {
		t.Fatal(err)
	}
	if err = solver.AddSoftObjective([]ast.ASTHandle{p, q, r}, []*big.Rat{big.NewRat(5, 1), big.NewRat(3, 1), big.NewRat(2, 1)}); err != nil {
		t.Fatal(err)
	}
	result, err := solver.Check()
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	if result.Status != lbool.True || !result.Optimal || len(result.Values) != 1 {
		t.Fatal("sortmax status", result.Status, result.Optimal)
	}
	value := result.Values[0]
	if !value.Soft || value.SoftCost.Cmp(big.NewRat(3, 1)) != 0 || value.SoftSatisfied.Cmp(big.NewRat(7, 1)) != 0 {
		t.Fatal("weighted mutex optimum", &value.SoftCost, &value.SoftSatisfied)
	}
	if len(result.MaxSMT) != 1 || result.MaxSMT[0].Engine != "sortmax" || result.MaxSMT[0].Inputs == 0 || result.MaxSMT[0].Auxiliaries == 0 || result.MaxSMT[0].NetworkClauses == 0 || !result.ModelFiltered {
		t.Fatal("SortMax statistics", result.MaxSMT)
	}
	modelEvaluator, err := model.NewModelEvaluator(result.Model, evaluator)
	if err != nil {
		t.Fatal(err)
	}
	defer modelEvaluator.Close()
	if err = modelEvaluator.SetModelCompletion(true); err != nil {
		t.Fatal(err)
	}
	for _, expression := range []ast.ASTHandle{p, q, r} {
		value, evalErr := modelEvaluator.EvaluateValue(expression)
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
			t.Fatal("source SortMax network leaves soft terms false in its filtered model")
		}
	}
}
