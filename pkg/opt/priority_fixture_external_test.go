package opt_test

// Shared retained-priority fixture; independent of MaxRes and SortMax tests.

import (
	"testing"

	_ "github.com/glycerine/g3/pkg/engine"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/cooperate"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func priorityFixture(t *testing.T) (*ast.ASTManager, model.ModelEvaluatorConfig) {
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
		_ = context.Del(&params)
		_ = cooperation.Close()
		_ = globals.Close()
		_ = catalog.Close()
		_ = context.Close()
		_ = manager.Close()
		_ = numbers.Close()
		_ = symbols.Close()
	})
	return manager, config
}

func priorityBool(t *testing.T, manager *ast.ASTManager, name string) ast.ASTHandle {
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
