package opt_test

import (
	"testing"

	_ "github.com/glycerine/g3/pkg/engine"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/cooperate"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type objectivePipelineFixture struct {
	manager *ast.ASTManager
	params  *uparams.ParamsRef
	config  smtcore.SMTSolverConfig
}

func newObjectivePipelineFixture(t *testing.T) objectivePipelineFixture {
	t.Helper()
	budget := arena.NewBudget(1 << 28)
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
	modelConfig := model.ModelEvaluatorConfig{Context: context, Globals: globals, Params: &params, Memory: numbers.RuntimeOwner(), Cooperation: cooperation}
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
		if used := budget.Used(); used != 0 {
			t.Errorf("retained pipeline budget %d", used)
		}
	})
	return objectivePipelineFixture{manager: manager, params: &params, config: smtcore.SMTSolverConfig{Params: context, Globals: globals, ModelEvaluator: modelConfig}}
}

func pipelineBoolean(t *testing.T, f objectivePipelineFixture, name string) ast.ASTHandle {
	t.Helper()
	sort, err := f.manager.MkBoolSort()
	if err != nil {
		t.Fatal(err)
	}
	symbol, err := f.manager.Symbols.Intern(name)
	if err != nil {
		t.Fatal(err)
	}
	value, err := f.manager.MkConst(symbol, sort)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
