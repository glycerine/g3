package pdr

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// Source: pdr::smt_context_manager::collect_statistics visits each solver in
// order. A Go error must not publish the completed prefix into the caller's
// statistics when a later real solver still owns an interrupted scope pop.
func TestSMTContextStatisticsRetryAfterNativePartialPop(t *testing.T) {
	for _, tier := range []string{"pool", "context"} {
		t.Run(tier, func(t *testing.T) { testSMTContextStatisticsRetry(t, tier) })
	}
}

func testSMTContextStatisticsRetry(t *testing.T, tier string) {
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
	config := smtcore.SMTSolverConfig{Params: params, Globals: globals}
	var contexts *SMTContextManager
	var upper *Context
	var collect func(*statistics.Statistics) error
	if tier == "context" {
		upper, err = NewContext(manager, nil, 0, ReachableNoCache, config)
		must(err)
		boolean, err := manager.MkBoolSort()
		must(err)
		name, err := symbols.Intern("native-statistics-cover-P")
		must(err)
		predicate, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
		must(err)
		property, err := manager.MkFreshConst("native-statistics-cover-property", boolean)
		must(err)
		must(upper.AddCover(0, predicate, property))
		// Borrow the actual child pool to exercise its exported methods; the
		// public cover created its real transformer and property ownership.
		contexts = upper.owner.contexts
		collect = upper.CollectStatistics
	} else {
		contexts, err = NewSMTContextManager(manager, nil, 0, config)
		must(err)
		collect = contexts.CollectStatistics
	}
	first, err := contexts.Fresh()
	must(err)
	second, err := contexts.Fresh()
	must(err)
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
		finish(second.Close)
		finish(first.Close)
		if upper != nil {
			finish(upper.Close)
		} else {
			finish(contexts.Close)
		}
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
			t.Fatal("statistics retry leaked storage", budget.Used())
		}
	}()
	status, err := first.Check(nil)
	must(err)
	if status != lbool.True {
		t.Fatal("healthy solver did not complete a real check", status)
	}
	count := func(values *statistics.Statistics, key statistics.Key) uint32 {
		var result uint32
		for i := 0; i < values.Size(); i++ {
			if values.Key(i) == key && values.IsUint(i) {
				result += values.UintValue(i)
			}
		}
		return result
	}
	before := statistics.New()
	must(collect(before))
	checks := count(before, "num checks")
	if checks == 0 {
		t.Fatal("first solver provided no check counter")
	}
	properties := uint32(0)
	if tier == "context" {
		properties = 1
		if got := count(before, "PDR num properties"); got != properties {
			t.Fatal("public cover did not contribute its real property", got)
		}
	}
	must(second.Push())

	// The assertion is the only owner of this large rational metadata after
	// lexical numeric owners are released. Popping its scope performs the real
	// native destructor, without a callback or private solver-state mutation.
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
	label, err := symbols.Intern("native-statistics-pop-assertion")
	must(err)
	declaration, err := manager.MkFuncDeclWithInfo(label, nil, boolean, info)
	must(err)
	formula, err := manager.MkApp(declaration, nil)
	must(err)
	must(second.Assert(formula))
	must(manager.DelFuncDeclInfo(&info))
	must(manager.DelParameter(&parameter))
	must(numbers.Del(&value))
	must(scoped.Close())
	if refs, err := manager.GetRefCount(declaration); err != nil || refs != 1 {
		t.Fatal("assertion is not sole metadata owner", refs, err)
	}

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
	stopped := second.Pop()
	if !errors.Is(stopped, arena.ErrBounds) || runtime.Stats().LiveBlocks >= blocks {
		t.Fatal("pop did not stop after native physical deallocation", stopped, blocks, runtime.Stats().LiveBlocks)
	}
	recovery, err = runtime.Allocate(4 << 20)
	must(err)
	destination := statistics.New()
	destination.UpdateUint("collector sentinel", 41)
	for attempt := 0; attempt < 2; attempt++ {
		cause := collect(destination)
		if !errors.Is(cause, ast.ErrASTPending) {
			t.Fatal("later solver did not refuse statistics during pending pop", cause)
		}
		if destination.Size() != 1 || count(destination, "collector sentinel") != 41 || count(destination, "num checks") != 0 || count(destination, "PDR num properties") != 0 {
			t.Errorf("failed collection %d published a completed prefix: size %d checks %d properties %d sentinel %d", attempt, destination.Size(), count(destination, "num checks"), count(destination, "PDR num properties"), count(destination, "collector sentinel"))
		}
	}
	must(second.Pop())
	must(collect(destination))
	if got := count(destination, "num checks"); got != checks || count(destination, "PDR num properties") != properties || count(destination, "collector sentinel") != 41 {
		t.Errorf("resumed collection duplicated prior prefix or lost sentinel: checks %d want %d; properties %d want %d; sentinel %d", got, checks, count(destination, "PDR num properties"), properties, count(destination, "collector sentinel"))
	}
}
