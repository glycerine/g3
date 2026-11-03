package spacer_test

// Source: virtual_solver destruction disables its guard once, then releases
// its buffered assertion owners. A refused native release must resume there.

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/muz/spacer"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestVirtualSolverNativeCleanupDoesNotDisableGuardTwice(t *testing.T) {
	must := func(err error) { t.Helper(); spacerMust(t, err) }
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
	backend, err := smtcore.NewSMTSolver(manager, nil, "QF_UF", smtcore.SMTSolverConfig{Params: params, Globals: globals})
	must(err)
	factory, err := spacer.NewVirtualSolverFactory(backend)
	must(err)
	view, err := factory.Fresh()
	must(err)
	var recovery memory.Block
	defer func() {
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
			t.Fatal("native cleanup did not converge")
		}
		finish(view.Close)
		finish(factory.Close)
		finish(manager.Close)
		finish(globals.Close)
		finish(catalog.Close)
		finish(params.Close)
		finish(numbers.Close)
		finish(symbols.Close)
		if recovery != (memory.Block{}) {
			if cause := runtime.Deallocate(&recovery); cause != nil && (!errors.Is(cause, arena.ErrBounds) || recovery != (memory.Block{})) {
				t.Fatal("recovery storage cleanup", cause)
			}
		}
		finish(runtime.Close)
		if budget.Used() != 0 {
			t.Fatal("virtual solver cleanup leaked storage", budget.Used())
		}
	}()

	// The buffered assertion is the sole owner of this real rational
	// declaration parameter; the backend never sees the asserted formula.
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
	name, err := symbols.Intern("virtual-native-cleanup-buffer")
	must(err)
	declaration, err := manager.MkFuncDeclWithInfo(name, nil, boolean, info)
	must(err)
	formula, err := manager.MkApp(declaration, nil)
	must(err)
	must(view.Assert(formula))
	must(manager.DelFuncDeclInfo(&info))
	must(manager.DelParameter(&parameter))
	must(numbers.Del(&value))
	must(scoped.Close())
	if references, cause := manager.GetRefCount(formula); cause != nil || references != 1 {
		t.Fatal("buffered assertion is not the sole formula owner", references, cause)
	}
	assertions, err := backend.NumAssertions()
	must(err)
	if assertions != 0 {
		t.Fatal("buffered assertion reached the backend before a check", assertions)
	}

	// Make the large numeral's physical deallocation cross the native TLS
	// accounting boundary after the guard assertion has already succeeded.
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
	before := runtime.Stats().LiveBlocks
	stopped := view.Close()
	if !errors.Is(stopped, arena.ErrBounds) || runtime.Stats().LiveBlocks >= before {
		t.Fatal("close did not stop after a native assertion release", stopped, before, runtime.Stats().LiveBlocks)
	}
	assertions, err = backend.NumAssertions()
	must(err)
	if assertions != 1 {
		t.Fatal("failed cleanup did not publish exactly one disabled guard", assertions)
	}
	disabled, err := backend.Assertion(0)
	must(err)
	if _, cause := view.CheckSat(nil); !errors.Is(cause, spacer.ErrVirtualSolverOwner) {
		t.Fatal("partly disposed view accepted solver work", cause)
	}
	recovery, err = runtime.Allocate(4 << 20)
	must(err)
	for retry := 0; retry < 2; retry++ {
		must(view.Close())
		count, err := backend.NumAssertions()
		must(err)
		guard, err := backend.Assertion(0)
		must(err)
		if count != assertions || guard != disabled {
			t.Fatal("cleanup retry replayed the disabled-guard assertion", retry, assertions, count)
		}
	}
	fresh, err := factory.Fresh()
	must(err)
	status, err := fresh.CheckSat(nil)
	must(err)
	if status != lbool.True {
		t.Fatal("completed disposal damaged a fresh shared view", status)
	}
	must(fresh.Close())
	must(factory.Close())
	must(factory.Close())
}
