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

// A public prequery context moves into the expression-query clone. Its live
// constraint cache simplifies an unsatisfiable temporary assertion; the input
// owner must transport and finish a native metadata destructor failure.
func TestPDRTransferredContextAssertionCleanupFailureAndRetry(t *testing.T) {
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
			t.Fatal("transferred assertion cleanup leaked storage", budget.Used())
		}
	}()

	for _, key := range []string{"fixedpoint.xform.slice", "fixedpoint.xform.inline_eager", "fixedpoint.xform.inline_linear", "fixedpoint.xform.subsumption_checker"} {
		must(globals.SourceSetName(key, "false"))
	}
	must(globals.SourceSetName("fixedpoint.pdr.cache_mode", "2"))
	must(globals.SourceSetName("fixedpoint.pdr.max_num_contexts", "0"))
	boolean, err := manager.MkBoolSort()
	must(err)
	name, err := symbols.Intern("native-transferred-assertion-P")
	must(err)
	predicate, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(engine.RegisterPredicate(predicate))
	fact, err := manager.MkApp(predicate, nil)
	must(err)
	must(engine.AddRule(fact, "native-transferred-assertion-fact"))

	_, err = engine.NumLevels(predicate)
	must(err)
	prequery := engine.pdr
	if prequery == nil {
		t.Fatal("no prequery context")
	}
	status, err := engine.QueryExpression(fact)
	must(err)
	if status != lbool.True || engine.queryClone == nil || engine.queryClone.pdr != prequery || engine.pdr != nil {
		t.Fatal("prequery context was not transferred to the expression clone", status)
	}
	context := engine.queryClone.pdr
	transformer, found, err := context.Transformer(predicate)
	must(err)
	if !found {
		t.Fatal("transformed predicate missing")
	}
	truth, err := manager.MkTrue()
	must(err)
	cached, err := transformer.IsCachedReachable(truth)
	must(err)
	if !cached {
		t.Fatal("real initialized cache lacks its fact")
	}

	// The raw conjunction keeps the metadata-bearing child until assertion
	// simplification. Its root is borrowed; only the live kernel input owner
	// protects it once the cache invokes Assert.
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
	label, err := symbols.Intern("native-transferred-assertion-metadata")
	must(err)
	declaration, err := manager.MkFuncDeclWithInfo(label, nil, boolean, info)
	must(err)
	formula, err := manager.MkApp(declaration, nil)
	must(err)
	must(manager.DelFuncDeclInfo(&info))
	must(manager.DelParameter(&parameter))
	must(numbers.Del(&value))
	must(scoped.Close())

	falsity, err := manager.MkFalse()
	must(err)
	formula, err = manager.MkFamilyApp(manager.BasicFamilyID(), ast.ASTOpAnd, nil, []ast.ASTHandle{formula, falsity})
	must(err)
	refs, err := manager.GetRefCount(formula)
	must(err)
	declRefs, err := manager.GetRefCount(declaration)
	must(err)
	if refs != 0 || declRefs != 1 {
		t.Fatal("cube metadata has unexpected owners", refs, declRefs)
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

	found, stopped := transformer.IsCachedReachable(formula)
	afterBlocks := runtime.Stats().LiveBlocks
	// Restore native accounting before checking the error so the ordinary
	// deferred teardown remains valid even against the pre-fix implementation.
	recovery, err = runtime.Allocate(4 << 20)
	must(err)
	if found || !errors.Is(stopped, arena.ErrBounds) || afterBlocks >= blocks {
		t.Fatal("assertion cleanup did not report a consumed native release", found, stopped, blocks, afterBlocks)
	}
	var receipt memory.ConstructionFailure
	if !errors.As(stopped, &receipt) {
		t.Fatal("assertion cleanup did not preserve its continuation", stopped)
	}
	shown, exposed := receipt.Error(), receipt.Unwrap()
	must(receipt.Cleanup())
	must(receipt.Cleanup())
	if receipt.Error() != shown || receipt.Unwrap() != exposed || receipt.CleanupCause() != nil {
		t.Fatal("assertion cleanup receipt changed after completion")
	}
	if engine.queryClone == nil || engine.queryClone.pdr != context {
		t.Fatal("cleanup discarded the live transferred context")
	}
	must(engine.CollectStatistics(statistics.New()))
	cached, err = transformer.IsCachedReachable(truth)
	must(err)
	if !cached {
		t.Fatal("completed assertion cleanup left a false constraint in the cache")
	}
	status, err = engine.QueryExpression(fact)
	must(err)
	if status != lbool.True || engine.queryClone == nil || engine.queryClone.pdr == context {
		t.Fatal("fresh expression query did not replace the prior owner", status)
	}
	if _, err = context.Initialized(); err == nil {
		t.Fatal("fresh query left its predecessor open")
	}
	must(engine.CollectStatistics(statistics.New()))
	must(engine.Close())
}
