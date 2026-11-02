package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type pdrLifecycleServices struct {
	manager  *ast.ASTManager
	globals  *uparams.GlobalParamsService
	catalog  *uparams.GlobalParamsCatalog
	params   *uparams.ParamsContext
	numerals *unumeral.RationalService
	symbols  *usymbol.Manager
}

func newPDRLifecycleServices(t *testing.T) (*ast.ASTManager, *uparams.GlobalParamsService) {
	return newPDRLifecycleServicesWithTrace(t, nil)
}

func newPDRLifecycleServicesWithTrace(t *testing.T, tags *diagnostic.TraceTags) (*ast.ASTManager, *uparams.GlobalParamsService) {
	t.Helper()
	services := &pdrLifecycleServices{}
	budget := arena.NewBudget(1 << 32)
	var err error
	services.symbols, err = usymbol.NewManager(usymbol.Config{Budget: budget})
	if err == nil {
		services.numerals, err = unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	}
	if err == nil {
		services.params, err = uparams.NewParamsContext(services.symbols, services.numerals, uparams.ParamsConfig{Budget: budget})
	}
	if err == nil {
		services.catalog, err = uparams.NewGlobalParamsCatalog(services.symbols, uparams.GlobalParamsCatalogConfig{Budget: budget})
	}
	if err == nil {
		services.globals, err = uparams.NewGlobalParamsService(services.symbols, services.params, uparams.GlobalParamsConfig{Budget: budget, Catalog: services.catalog})
	}
	if err == nil {
		err = services.globals.InitializeSource()
	}
	if err == nil {
		services.manager, err = ast.NewASTManager(ast.ASTManagerConfig{TraceTags: tags, Symbols: services.symbols, Numerals: services.numerals, Budget: budget})
	}
	if err == nil {
		err = services.manager.RegisterDeclarationPlugins()
	}
	if err != nil {
		_ = services.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := services.Close(); err != nil {
			t.Fatal(err)
		}
	})
	return services.manager, services.globals
}

func (s *pdrLifecycleServices) Close() (result error) {
	if s.manager != nil {
		result = errors.Join(result, s.manager.Close())
		s.manager = nil
	}
	if s.globals != nil {
		result = errors.Join(result, s.globals.Close())
		s.globals = nil
	}
	if s.catalog != nil {
		result = errors.Join(result, s.catalog.Close())
		s.catalog = nil
	}
	if s.params != nil {
		result = errors.Join(result, s.params.Close())
		s.params = nil
	}
	if s.numerals != nil {
		result = errors.Join(result, s.numerals.Close())
		s.numerals = nil
	}
	if s.symbols != nil {
		result = errors.Join(result, s.symbols.Close())
		s.symbols = nil
	}
	return result
}
