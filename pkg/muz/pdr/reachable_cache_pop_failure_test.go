package pdr

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// The lookup is unsatisfiable, so its result is recorded before popping the
// temporary candidate. Its metadata destructor fails after consuming storage.
func TestReachableConstraintCacheRecoversPartialPop(t *testing.T) {
	for _, closeOnly := range []bool{false, true} {
		name := "retry"
		if closeOnly {
			name = "close"
		}
		t.Run(name, func(t *testing.T) { reachableCachePartialPop(t, closeOnly) })
	}
}

func reachableCachePartialPop(t *testing.T, closeOnly bool) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
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
	contexts, err := NewSMTContextManager(manager, nil, 0, smtcore.SMTSolverConfig{Params: params, Globals: globals})
	must(err)
	truth, err := manager.MkTrue()
	must(err)
	cache, err := NewReachableCache(manager, contexts, truth, ReachableConstraintCache)
	must(err)

	// The false initial disjunct makes the assumption unsatisfiable without
	// simplifying away the temporary assertion.
	falsity, err := manager.MkFalse()
	must(err)
	must(cache.AddInit(falsity))
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
	boolean, err := manager.MkBoolSort()
	must(err)
	label, err := symbols.Intern("reachable-scope-pop-assertion")
	must(err)
	declaration, err := manager.MkFuncDeclWithInfo(label, nil, boolean, info)
	must(err)
	formula, err := manager.MkApp(declaration, nil)
	must(err)
	must(manager.DelFuncDeclInfo(&info))
	must(manager.DelParameter(&parameter))
	must(numbers.Del(&value))
	must(scoped.Close())

	// Move native accounting to the boundary that makes metadata cleanup fail
	// after physically releasing a block. Restore it before resuming the pop.
	a, err := runtime.Allocate(8)
	must(err)
	b, err := runtime.Allocate(8)
	must(err)
	delta := int64(-1000000)
	if err = runtime.Reallocate(&a, uint64(16)+uint64(delta)-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("first boundary", err)
	}
	must(runtime.Deallocate(&a))
	delta = math.MinInt64 + 262144 - runtime.State().GlobalSize + 16
	if err = runtime.Reallocate(&b, uint64(delta)+16-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("second boundary", err)
	}
	must(runtime.Deallocate(&b))
	blocks := runtime.Stats().LiveBlocks
	found, failure := cache.IsReachable(formula)
	if !errors.Is(failure, arena.ErrBounds) || runtime.Stats().LiveBlocks >= blocks {
		t.Fatal("scope pop did not fail after native cleanup", failure)
	}
	level, levelErr := cache.context.solver.ScopeLevel()
	if found || levelErr != nil || level != 1 || !cache.pendingPop || cache.stats.Misses != 1 {
		t.Fatal("lookup did not retain its failed pop", found, level, levelErr, cache.pendingPop, cache.stats)
	}
	recovery, err := runtime.Allocate(4 << 20)
	must(err)
	if !closeOnly {
		found, err = cache.IsReachable(truth)
		must(err)
		if found || cache.pendingPop {
			t.Fatal("retry retained temporary scope", found, cache.pendingPop)
		}
		level, err = cache.context.solver.ScopeLevel()
		must(err)
		if level != 0 {
			t.Fatal("retry did not retire lookup scope", level)
		}
		must(cache.AddReachable(truth))
		found, err = cache.IsReachable(truth)
		must(err)
		if !found {
			t.Fatal("cache did not accept work after pop retry")
		}
	}
	must(cache.Close())
	must(contexts.Close())
	must(manager.Close())
	must(globals.Close())
	must(catalog.Close())
	must(params.Close())
	must(numbers.Close())
	must(symbols.Close())
	if err = runtime.Deallocate(&recovery); err != nil && (!errors.Is(err, arena.ErrBounds) || recovery != (memory.Block{})) {
		t.Fatal(err)
	}
	must(runtime.Close())
	if budget.Used() != 0 {
		t.Fatal("leak", budget.Used())
	}
}
