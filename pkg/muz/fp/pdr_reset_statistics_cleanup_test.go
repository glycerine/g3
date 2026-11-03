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

// Source context::reset (pdr_context.cpp:1475-1486) leaves its manager's
// kernels and their cumulative statistics alive. Go archives those counters
// when retiring the old program's pools, including interrupted destruction.
func TestPDRResetPreservesStatisticsAcrossNativeCleanupFailure(t *testing.T) {
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
			t.Fatal("PDR reset cleanup leaked storage", budget.Used())
		}
	}()

	for _, key := range []string{"fixedpoint.xform.slice", "fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear", "fixedpoint.xform.subsumption_checker"} {
		must(globals.SourceSetName(key, "false"))
	}
	must(globals.SourceSetName("fixedpoint.pdr.cache_mode", "0"))
	must(globals.SourceSetName("fixedpoint.pdr.max_num_contexts", "0"))
	boolean, err := manager.MkBoolSort()
	must(err)
	name, err := symbols.Intern("native-reset-P")
	must(err)
	predicate, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(engine.RegisterPredicate(predicate))
	fact, err := manager.MkApp(predicate, nil)
	must(err)
	must(engine.AddRule(fact, "native-reset-fact"))

	_, err = engine.NumLevels(predicate)
	must(err)
	prequery := engine.pdr
	if prequery == nil {
		t.Fatal("no prequery context")
	}
	status, err := engine.QueryPredicate(predicate)
	must(err)
	if status != lbool.True || engine.pdr != prequery || engine.queryClone != nil {
		t.Fatal("direct query did not install its public prequery context", status)
	}
	context := engine.pdr

	checkCount := func(values *statistics.Statistics) uint32 {
		var count uint32
		for i := 0; i < values.Size(); i++ {
			if values.Key(i) == "num checks" && values.IsUint(i) {
				count += values.UintValue(i)
			}
		}
		return count
	}
	checks := func() (uint32, error) {
		values := statistics.New()
		if err := context.CollectStatistics(values); err != nil {
			return 0, err
		}
		return checkCount(values), nil
	}
	beforeChecks, err := checks()
	must(err)
	if beforeChecks == 0 {
		t.Fatal("fixture produced no real solver checks")
	}
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
	label, err := symbols.Intern("native-reset-metadata")
	must(err)
	declaration, err := manager.MkFuncDeclWithInfo(label, nil, boolean, info)
	must(err)
	formula, err := manager.MkApp(declaration, nil)
	must(err)
	must(context.AddCover(-1, predicate, formula))
	must(manager.DelFuncDeclInfo(&info))
	must(manager.DelParameter(&parameter))
	must(numbers.Del(&value))
	must(scoped.Close())

	refs, err := manager.GetRefCount(formula)
	must(err)
	declRefs, err := manager.GetRefCount(declaration)
	must(err)
	if refs == 0 || declRefs != 1 {
		t.Fatal("cover metadata has unexpected owners", refs, declRefs)
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

	stopped := context.Reset()
	afterBlocks := runtime.Stats().LiveBlocks
	// Restore native headroom before inspecting results or resuming owners.
	recovery, err = runtime.Allocate(4 << 20)
	must(err)
	if !errors.Is(stopped, arena.ErrBounds) || afterBlocks >= blocks {
		t.Fatal("reset did not reach consumed native metadata release", stopped, blocks, afterBlocks)
	}
	// The partially closed transformers cannot publish their ordinary gauges.
	// The independent pool snapshot must still report its exact contribution.
	pending, err := context.SnapshotStatistics()
	must(err)
	if count := checkCount(&pending.Solvers); count != beforeChecks {
		t.Fatal("interrupted reset lost its pre-close kernel snapshot", beforeChecks, count)
	}
	var retryErr error
	for retry := 0; retry < 3; retry++ {
		retryErr = context.Reset()
		if retryErr == nil {
			break
		}
		if !errors.Is(retryErr, ast.ErrASTPending) {
			t.Fatal("reset retry failed outside the unfinished native release", retryErr)
		}
	}
	must(retryErr)
	for repeat := 0; repeat < 3; repeat++ {
		afterChecks, err := checks()
		must(err)
		if afterChecks != beforeChecks {
			t.Fatal("reset lost or duplicated archived solver statistics", beforeChecks, afterChecks)
		}
		must(context.Reset())
	}
	must(context.Initialize(engine.pdrRules))
	status, err = context.Solve(engine.pdrQuery)
	must(err)
	if status != lbool.True {
		t.Fatal("reinitialized program no longer reaches its fact", status)
	}
	freshChecks, err := checks()
	must(err)
	if freshChecks != 2*beforeChecks {
		t.Fatal("fresh solve did not add exactly its own kernel checks", beforeChecks, freshChecks)
	}
	must(context.Reset())
	must(context.ResetStatistics())
	finalChecks, err := checks()
	must(err)
	if finalChecks != freshChecks {
		t.Fatal("later reset changed source cumulative kernel counters", freshChecks, finalChecks)
	}
	must(engine.Close())
}
