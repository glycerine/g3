package opt

// Test fixture shared by the internal MaxRes and WMax tests: an AST manager
// with the pseudo-Boolean plugin, parameter services and a model evaluator
// configuration.

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/cooperate"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type maxSMTTestConfig struct {
	Context *uparams.ParamsContext
	Globals *uparams.GlobalParamsService
	Model   model.ModelEvaluatorConfig
}

func maxSMTTestFixture(t *testing.T) (*ast.ASTManager, maxSMTTestConfig) {
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
	if _, err = manager.RegisterPseudoBooleanPlugin(); err != nil {
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
	local := context.EmptyValue()
	config := maxSMTTestConfig{Context: context, Globals: globals, Model: model.ModelEvaluatorConfig{Context: context, Globals: globals, Params: &local, Memory: numbers.RuntimeOwner(), Cooperation: cooperation}}
	t.Cleanup(func() {
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
