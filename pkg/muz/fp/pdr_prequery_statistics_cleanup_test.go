package fp

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// A prequery context is a live owner created by the public level/cover API.
// Parameter replacement snapshots its statistics before closing the context;
// retry must not ask a partly closed statistics provider to restart that step.
func TestPDRPrequeryParameterUpdateResumesAfterNativeBackgroundCleanup(t *testing.T) {
	for _, completion := range []string{"parameter-update", "expression-query", "statistics-reset"} {
		t.Run(completion, func(t *testing.T) { testPDRPrequeryStatisticsCleanup(t, completion) })
	}
}

func testPDRPrequeryStatisticsCleanup(t *testing.T, completion string) {
	must := func(err error) { t.Helper(); fragmentMust(t, err) }
	budget := arena.NewBudget(1 << 29)
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, ThreadLocal: true, SizeTBits: 64})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget, PageSize: 1}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
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
	engine, err := NewPropositionalEngineWithServices(manager, params, globals)
	must(err)
	must(engine.SelectEngine("pdr"))
	var recovery memory.Block
	defer func() {
		finish := func(close func() error) {
			t.Helper()
			for i := 0; i < 128; i++ {
				before := runtime.Stats().LiveBlocks
				err := close()
				if err == nil {
					return
				}
				if !errors.Is(err, arena.ErrBounds) || runtime.Stats().LiveBlocks >= before {
					t.Fatal("cleanup made no physical progress", err, runtime.State())
				}
			}
			t.Fatal("cleanup horizon")
		}
		finish(engine.Close)
		finish(manager.Close)
		finish(globals.Close)
		finish(catalog.Close)
		finish(params.Close)
		finish(numbers.Close)
		finish(symbols.Close)
		if recovery != (memory.Block{}) {
			if err := runtime.Deallocate(&recovery); err != nil && (!errors.Is(err, arena.ErrBounds) || recovery != (memory.Block{})) {
				t.Fatal("recovery storage cleanup", err)
			}
		}
		finish(runtime.Close)
		if budget.Used() != 0 {
			t.Fatal("prequery statistics cleanup leaked storage", budget.Used())
		}
	}()
	boolean, err := manager.MkBoolSort()
	must(err)
	name, err := symbols.Intern("native-prequery-statistics-P")
	must(err)
	predicate, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(engine.RegisterPredicate(predicate))
	fact, err := manager.MkApp(predicate, nil)
	must(err)
	must(engine.addRuleEager(fact, "native-prequery-statistics-fact"))

	// The manager's background becomes the only owner of a real, large
	// rational declaration parameter after the parent background is replaced.
	scoped, err := numeral.NewScopedInt(numbers.Manager.Z)
	must(err)
	integer, err := scoped.Get()
	must(err)
	words := make([]uint32, 262144)
	words[0], words[len(words)-1] = 17, 1
	must(numbers.Manager.Z.SetLimbs(integer, words))
	value, err := numbers.NewFromScopedInt(scoped)
	must(err)
	parameter, err := manager.NewParameterRational(&value)
	must(err)
	info, err := manager.NewFuncDeclInfo(ast.NullFamilyID, 917, []ast.ASTParameter{parameter}, ast.ASTFuncFlags{}, false)
	must(err)
	label, err := symbols.Intern("native-prequery-background")
	must(err)
	declaration, err := manager.MkFuncDeclWithInfo(label, nil, boolean, info)
	must(err)
	formula, err := manager.MkApp(declaration, nil)
	must(err)
	must(engine.SetBackground(formula))
	must(manager.DelFuncDeclInfo(&info))
	must(manager.DelParameter(&parameter))
	must(numbers.Del(&value))
	must(scoped.Close())
	levels, err := engine.NumLevels(predicate)
	must(err)
	context := engine.pdr
	if levels != 0 || context == nil || engine.pdrRules != nil {
		t.Fatal("public levels call did not create a prequery context", levels)
	}
	truth, err := manager.MkTrue()
	must(err)
	must(engine.SetBackground(truth))
	if refs, err := manager.GetRefCount(formula); err != nil || refs != 1 {
		t.Fatal("prequery manager is not sole background owner", refs, err)
	}

	// Reach a native deallocation error after a physical block is freed.
	a, err := runtime.Allocate(8)
	must(err)
	b, err := runtime.Allocate(8)
	must(err)
	delta := int64(-1000000)
	if err = runtime.Reallocate(&a, uint64(16)+uint64(delta)-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("first TLS boundary", err)
	}
	must(runtime.Deallocate(&a))
	delta = math.MinInt64 + 262144 - runtime.State().GlobalSize + 16
	if err = runtime.Reallocate(&b, uint64(delta)+16-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("second TLS boundary", err)
	}
	must(runtime.Deallocate(&b))
	if state := runtime.State(); state.GlobalSize != math.MinInt64+262144 || state.LocalSize != 0 {
		t.Fatal("native TLS fixture", state)
	}
	blocks := runtime.Stats().LiveBlocks
	stopped := engine.SetQueryParameters(nil)
	if !errors.Is(stopped, arena.ErrBounds) || runtime.Stats().LiveBlocks >= blocks || engine.pdr != context {
		t.Fatal("did not stop inside actual prequery context destruction", stopped, blocks, runtime.Stats().LiveBlocks, engine.pdr == context)
	}
	recovery, err = runtime.Allocate(4 << 20)
	must(err)
	// Collection must use the complete pre-destruction snapshot while the
	// original manager refuses new work during its unfinished cleanup.
	must(engine.CollectStatistics(statistics.New()))
	switch completion {
	case "parameter-update":
		firstRetry := engine.SetQueryParameters(nil)
		secondRetry := engine.SetQueryParameters(nil)
		if firstRetry != nil || secondRetry != nil {
			t.Fatalf("parameter retry revisited partially closed statistics: first %v; second %v", firstRetry, secondRetry)
		}
	case "statistics-reset":
		must(engine.ResetStatistics())
	}
	if completion != "expression-query" && (engine.pdr != nil || engine.pdrDisposal != nil) {
		t.Fatal("successful mutation left the retired context or disposal installed")
	}
	// The expression-query completion has no explicit cleanup retry before
	// this call: disposal must finish before prequery-context transfer.
	status, err := engine.QueryExpression(fact)
	must(err)
	if status != lbool.True {
		t.Fatal("fresh query did not recover the original fact", status)
	}
	if engine.pdrDisposal != nil || engine.pdr == context || engine.queryClone != nil && engine.queryClone.pdr == context {
		t.Fatal("fresh query reused the partly destroyed prequery context")
	}
	if _, err := context.Initialized(); err == nil {
		t.Fatal("fresh query left its predecessor open")
	}
}
