package fp

// Source: spacer_dl_interface::updt_params owns context destruction before
// replacement. A failed Go destruction must finish before query-owner transfer.

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/spacer"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSpacerPrequeryDisposalResumesBeforePublicRetry(t *testing.T) {
	for _, completion := range []string{"expression-query", "parameter-update", "statistics-reset"} {
		t.Run(completion, func(t *testing.T) { testSpacerPrequeryDisposalRetry(t, completion) })
	}
}

func testSpacerPrequeryDisposalRetry(t *testing.T, completion string) {
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	numeralBudget := arena.NewBudget(1 << 26)
	paramsBudget := arena.NewBudget(1 << 24)
	symbolBudget := arena.NewBudget(1 << 24)
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: numeralBudget, SizeTBits: 64, ThreadLocal: true})
	must(err)
	numerals, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{PageSize: 2, Budget: numeralBudget}, Words: arena.VectorConfig{PageSize: 16, Descriptors: arena.Config{PageSize: 2, Budget: numeralBudget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: symbolBudget, PageSize: 64, DescriptorPageSize: 2})
	must(err)
	params, err := uparams.NewParamsContext(symbols, numerals, uparams.ParamsConfig{Budget: paramsBudget, PageEntries: 1, Text: arena.VectorConfig{PageSize: 16, Descriptors: arena.Config{PageSize: 2, Budget: paramsBudget}}})
	must(err)
	finish := func(close func() error) {
		t.Helper()
		for i := 0; i < 128; i++ {
			before := runtime.Stats().LiveBlocks
			cause := close()
			if cause == nil {
				return
			}
			if !errors.Is(cause, arena.ErrBounds) || runtime.Stats().LiveBlocks >= before {
				t.Fatal("cleanup made no physical progress", cause, runtime.State())
			}
		}
		t.Fatal("owner cleanup did not converge")
	}
	var recovery memory.Block
	defer func() {
		finish(params.Close)
		finish(numerals.Close)
		finish(symbols.Close)
		if recovery != (memory.Block{}) {
			if cause := runtime.Deallocate(&recovery); cause != nil && (!errors.Is(cause, arena.ErrBounds) || recovery != (memory.Block{})) {
				t.Fatal("recovery storage cleanup", cause)
			}
		}
		finish(runtime.Close)
		if numeralBudget.Used() != 0 || paramsBudget.Used() != 0 || symbolBudget.Used() != 0 {
			t.Fatal("pre-query disposal leaked bytes", numeralBudget.Used(), paramsBudget.Used(), symbolBudget.Used())
		}
	}()

	// The manager copies the query parameters. Its independent rational
	// destruction can stop after the empty pre-query solver pools close.
	module, err := params.NewValue()
	must(err)
	integers := params.NumeralsField.Manager.Z
	scoped, err := numeral.NewScopedInt(integers)
	must(err)
	integer, err := scoped.Get()
	must(err)
	digits := make([]uint32, 32768)
	digits[0], digits[len(digits)-1] = 17, 1
	must(integers.SetLimbs(integer, digits))
	rational, err := params.NumeralsField.NewFromScopedInt(scoped)
	must(err)
	must(scoped.Close())
	parameterName := "spacer-prequery-disposal-numeral"
	must(params.SetRatName(&module, &parameterName, &rational))
	must(params.NumeralsField.Del(&rational))
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: paramsBudget})
	must(err)
	must(manager.RegisterDeclarationPlugins())
	catalog, err := uparams.NewGlobalParamsCatalog(symbols, uparams.GlobalParamsCatalogConfig{Budget: paramsBudget})
	must(err)
	globals, err := uparams.NewGlobalParamsService(symbols, params, uparams.GlobalParamsConfig{Budget: paramsBudget, Catalog: catalog})
	must(err)
	must(globals.InitializeSource())
	engine, err := NewPropositionalEngineWithServices(manager, params, globals)
	must(err)
	defer func() {
		finish(engine.Close)
		finish(manager.Close)
		finish(globals.Close)
		finish(catalog.Close)
	}()
	must(engine.SelectEngine("spacer"))
	must(engine.SetQueryParameters(&module))
	must(params.Del(&module))
	boolean, err := manager.MkBoolSort()
	must(err)
	name, err := symbols.Intern("prequery-disposal-retry")
	must(err)
	predicate, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(engine.RegisterPredicate(predicate))
	fact, err := manager.MkApp(predicate, nil)
	must(err)
	must(engine.AddRule(fact, "prequery-disposal-fact"))
	levels, err := engine.NumLevels(predicate)
	must(err)
	prequery := engine.spacer
	if levels != 0 || prequery == nil || engine.spacerRules != nil {
		t.Fatal("NumLevels did not create an uninitialized pre-query context", levels)
	}

	a, err := runtime.Allocate(8)
	must(err)
	b, err := runtime.Allocate(8)
	must(err)
	delta := int64(-1000000)
	if err = runtime.Reallocate(&a, uint64(16)+uint64(delta)-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("runtime refusal priming", err)
	}
	must(runtime.Deallocate(&a))
	delta = math.MinInt64 + 1000 - runtime.State().GlobalSize + 16
	if err = runtime.Reallocate(&b, uint64(delta)+16-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("runtime lower-bound priming", err)
	}
	must(runtime.Deallocate(&b))
	integerBudget := integers.BudgetField
	integerBudget.FailAfter(0)
	paramsBudget.FailAfter(0)
	stopped := engine.SetQueryParameters(nil)
	integerBudget.FailAfter(-1)
	paramsBudget.FailAfter(-1)
	if !errors.Is(stopped, arena.ErrBounds) || engine.spacer != prequery || engine.spacerDisposal == nil {
		t.Fatal("parameter update lost its stopped pre-query disposal", stopped)
	}
	if _, cause := engine.NumLevels(predicate); !errors.Is(cause, ast.ErrASTPending) {
		t.Fatal("property lookup exposed the partly destroyed context", cause)
	}
	recovery, err = runtime.Allocate(4 << 20)
	must(err)
	must(engine.CollectStatistics(statistics.New()))
	switch completion {
	case "parameter-update":
		must(engine.SetQueryParameters(nil))
		must(engine.SetQueryParameters(nil))
	case "statistics-reset":
		must(engine.ResetStatistics())
		must(engine.ResetStatistics())
	}
	if completion != "expression-query" && (engine.spacer != nil || engine.spacerDisposal != nil) {
		t.Fatal("completed cleanup left the old pre-query disposal installed")
	}
	// The expression-query case has no explicit cleanup retry before this
	// public call: it must retire the old owner before transferring query state.
	status, err := engine.QueryExpression(fact)
	must(err)
	if status != lbool.True {
		t.Fatal("query retry lost the original reachable fact", status)
	}
	if engine.spacerDisposal != nil || engine.spacer == prequery || engine.queryClone != nil && engine.queryClone.spacer == prequery {
		t.Fatal("query retry reused the partly destroyed context")
	}
	if _, cause := prequery.Initialized(); !errors.Is(cause, spacer.ErrContextOwner) {
		t.Fatal("query retry left its predecessor open", cause)
	}
	must(engine.CollectStatistics(statistics.New()))
	must(engine.clearQueryResources())
	must(engine.clearQueryResources())
	must(engine.Close())
	must(engine.Close())
}
