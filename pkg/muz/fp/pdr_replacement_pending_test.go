package fp

// Source: pdr_dl_interface::~dl_interface/updt_params ownership. A native
// release refusal must keep the concrete PDR context reachable for retry.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/pdr"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestPDRReplacementPreservesContextWhenNativeCleanupRefuses(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var engine *PropositionalEngine
	var construction *pdr.Context
	constructionCause := errors.New("configured PDR construction failed")
	var constructionCleanup memory.ConstructionFailure
	var runtime *memory.Runtime
	var err error
	active, callback := false, false
	var closeErr error
	runtime, err = memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active && !callback {
			callback = true
			original := engine.pdr
			revision := engine.pdrRevision
			unfoldings, depth := engine.pdrStatistics.Unfoldings, engine.pdrStatistics.MaxDepth
			statisticsSize := engine.pdrStatistics.Solvers.Size()
			closeErr = engine.closePDRForReplacement()
			if !errors.Is(closeErr, ast.ErrASTPending) || engine.pdr != original || engine.pdrRevision != revision || engine.pdrStatistics.Unfoldings != unfoldings || engine.pdrStatistics.MaxDepth != depth || engine.pdrStatistics.Solvers.Size() != statisticsSize {
				t.Fatalf("refused replacement discarded PDR owner: %v", closeErr)
			}
			returned, failure := failPDRContextConstruction(construction, runtime, constructionCause)
			if returned != nil || !errors.Is(failure, constructionCause) || !errors.Is(failure, ast.ErrASTPending) || !errors.As(failure, &constructionCleanup) {
				t.Fatalf("refused construction cleanup discarded PDR owner: %v", failure)
			}
			if constructionCleanup.RuntimeOwner() != runtime || !errors.Is(constructionCleanup.OriginalCause(), constructionCause) || !errors.Is(constructionCleanup.CleanupCause(), ast.ErrASTPending) {
				t.Fatal("construction failure did not expose its concrete causes")
			}
		}
		return f
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	params, err := uparams.NewParamsContext(symbols, numbers, uparams.ParamsConfig{Budget: budget})
	must(err)
	catalog, err := uparams.NewGlobalParamsCatalog(symbols, uparams.GlobalParamsCatalogConfig{Budget: budget})
	must(err)
	globals, err := uparams.NewGlobalParamsService(symbols, params, uparams.GlobalParamsConfig{Budget: budget, Catalog: catalog})
	must(err)
	must(globals.InitializeSource())
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	must(manager.RegisterDeclarationPlugins())
	defer func() {
		active = false
		must(runtime.SetMaxCount(0))
		if engine != nil {
			must(engine.Close())
		}
		if construction != nil && constructionCleanup == nil {
			must(construction.Close())
		}
		must(manager.Close())
		must(globals.Close())
		must(catalog.Close())
		must(params.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("PDR replacement fixture leaked bytes", budget.Used())
		}
	}()
	engine, err = NewPropositionalEngineWithServices(manager, params, globals)
	must(err)
	must(engine.SelectEngine("pdr"))
	view, err := engine.fixedpointParamView()
	must(err)
	construction, err = engine.newConfiguredPDRContext(view)
	must(errors.Join(err, view.Close()))
	context, err := engine.pdrContext()
	must(err)
	if context == nil || engine.pdr == nil || engine.pdrRevision == 0 {
		t.Fatal("fixture did not install a PDR context")
	}
	value, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&value)) }()
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	if body == nil || !callback || !errors.Is(closeErr, ast.ErrASTPending) {
		t.Fatal("numerical callback did not refuse PDR cleanup", body, callback, closeErr)
	}
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	must(constructionCleanup.Cleanup())
	must(constructionCleanup.Cleanup())
	if constructionCleanup.CleanupCause() != nil {
		t.Fatal("completed construction cleanup kept a stale failure")
	}
	if _, err := construction.Initialized(); err == nil {
		t.Fatal("construction cleanup retry left context open")
	}
	must(engine.closePDRForReplacement())
	must(engine.closePDRForReplacement())
	if engine.pdr != nil || engine.pdrRevision != 0 || engine.pdrRuleRevision != 0 {
		t.Fatal("replacement retry did not retire PDR state")
	}
}
