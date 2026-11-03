package fp

// Source spacer_dl_interface.cpp:45-49 constructs context::m_pm; completed
// manager members outlive a failed later constructor until destruction finishes.
// params.cpp:559-583 copies the rational into the nonempty solver parameter set.

import (
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestSpacerManagerConstructionNativeCleanupAndRetry(t *testing.T) {
	sawFailure, sawNativeFailure, sawSuccess := false, false, false
	for allowance := int64(0); allowance < 128; allowance++ {
		succeeded, nativeFailure := false, false
		if !t.Run(fmt.Sprint(allowance), func(t *testing.T) {
			succeeded, nativeFailure = testSpacerManagerConstructionNativeCleanup(t, allowance)
		}) {
			return
		}
		sawNativeFailure = sawNativeFailure || nativeFailure
		if succeeded {
			sawSuccess = true
			break
		}
		sawFailure = true
	}
	if !sawFailure || !sawNativeFailure || !sawSuccess {
		t.Fatal("constructor campaign missed allocation failure, native cleanup refusal or completion", sawFailure, sawNativeFailure, sawSuccess)
	}
}

func testSpacerManagerConstructionNativeCleanup(t *testing.T, allowance int64) (bool, bool) {
	numeralBudget := arena.NewBudget(1 << 26)
	paramsBudget := arena.NewBudget(1 << 24)
	symbolBudget := arena.NewBudget(1 << 24)
	runtime, err := umemory.NewRuntime(umemory.RuntimeConfig{Budget: numeralBudget, SizeTBits: 64, ThreadLocal: true})
	fragmentMust(t, err)
	numerals, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{Memory: runtime, Profile: umemory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{PageSize: 2, Budget: numeralBudget}, Words: arena.VectorConfig{PageSize: 16, Descriptors: arena.Config{PageSize: 2, Budget: numeralBudget}}}})
	fragmentMust(t, err)
	symbols, err := usymbol.NewManager(usymbol.Config{Budget: symbolBudget, PageSize: 64, DescriptorPageSize: 2})
	fragmentMust(t, err)
	context, err := uparams.NewParamsContext(symbols, numerals, uparams.ParamsConfig{Budget: paramsBudget, PageEntries: 1, Text: arena.VectorConfig{PageSize: 16, Descriptors: arena.Config{PageSize: 2, Budget: paramsBudget}}})
	fragmentMust(t, err)
	finish := func(close func() error) {
		for i := 0; i < 64; i++ {
			cause := close()
			if cause == nil {
				return
			}
			if !errors.Is(cause, arena.ErrBounds) {
				t.Fatal(cause)
			}
		}
		t.Fatal("owner cleanup did not converge")
	}
	defer func() {
		finish(context.Close)
		finish(numerals.Close)
		finish(runtime.Close)
		fragmentMust(t, symbols.Close())
		if numeralBudget.Used() != 0 || paramsBudget.Used() != 0 || symbolBudget.Used() != 0 {
			t.Fatal("Spacer construction cleanup leaked bytes", numeralBudget.Used(), paramsBudget.Used(), symbolBudget.Used())
		}
	}()
	module, err := context.NewValue()
	fragmentMust(t, err)
	name := "spacer-constructor-large-numeral"
	integers := context.NumeralsField.Manager.Z
	scoped, err := unumeral.NewScopedInt(integers)
	fragmentMust(t, err)
	value, err := scoped.Get()
	fragmentMust(t, err)
	digits := make([]uint32, 65536)
	digits[0], digits[len(digits)-1] = 17, 1
	fragmentMust(t, integers.SetLimbs(value, digits))
	rational, err := context.NumeralsField.NewFromScopedInt(scoped)
	fragmentMust(t, err)
	fragmentMust(t, scoped.Close())
	fragmentMust(t, context.SetRatName(&module, &name, &rational))
	fragmentMust(t, context.NumeralsField.Del(&rational))
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: paramsBudget})
	fragmentMust(t, err)
	fragmentMust(t, manager.RegisterDeclarationPlugins())
	catalog, err := uparams.NewGlobalParamsCatalog(symbols, uparams.GlobalParamsCatalogConfig{Budget: paramsBudget})
	fragmentMust(t, err)
	globals, err := uparams.NewGlobalParamsService(symbols, context, uparams.GlobalParamsConfig{Budget: paramsBudget, Catalog: catalog})
	fragmentMust(t, err)
	fragmentMust(t, globals.InitializeSource())
	engine, err := NewPropositionalEngineWithServices(manager, context, globals)
	fragmentMust(t, err)
	defer func() {
		fragmentMust(t, engine.Close())
		finish(manager.Close)
		fragmentMust(t, globals.Close())
		fragmentMust(t, catalog.Close())
	}()
	fragmentMust(t, engine.SelectEngine("spacer"))
	fragmentMust(t, engine.SetQueryParameters(&module))
	fragmentMust(t, context.Del(&module))
	boolean, err := manager.MkBoolSort()
	fragmentMust(t, err)
	prequeryName, err := symbols.Intern("constructor-stopped-decrement")
	fragmentMust(t, err)
	predicate, err := manager.MkFuncDecl(prequeryName, nil, boolean, ast.ASTFuncDeclInfo{})
	fragmentMust(t, err)
	fragmentMust(t, engine.RegisterPredicate(predicate))

	// Leave a negative, unsynchronized TLS remainder before the actual deep
	// copy. Its allocation can synchronize, but an interrupted constructor's
	// native destructor crosses the signed accounting boundary on release.
	ballast, err := runtime.Allocate(50000)
	fragmentMust(t, err)
	first, err := runtime.Allocate(8)
	fragmentMust(t, err)
	second, err := runtime.Allocate(8)
	fragmentMust(t, err)
	delta := int64(-1000000)
	if err = runtime.Reallocate(&first, uint64(16)+uint64(delta)-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("first native accounting adjustment", err)
	}
	fragmentMust(t, runtime.Deallocate(&first))
	delta = math.MinInt64 + 1000 - runtime.State().GlobalSize + 16
	if err = runtime.Reallocate(&second, uint64(delta)+16-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("native lower accounting boundary", err)
	}
	fragmentMust(t, runtime.Deallocate(&second))
	fragmentMust(t, runtime.Deallocate(&ballast))
	if state := runtime.State(); state.GlobalSize != math.MinInt64+1000 || state.LocalSize >= 0 {
		t.Fatal("missing negative native remainder", state)
	}
	before, refs := context.Stats(), manager.Stats().LiveReferences
	paramsBudget.FailAfter(allowance)
	levels, cause := engine.NumLevels(predicate)
	paramsBudget.FailAfter(-1)
	// Repair accounting without completing any returned construction receipt.
	recovery, err := runtime.Allocate(4 << 20)
	fragmentMust(t, err)
	state := runtime.State()
	delta = int64(-500000) - state.GlobalSize - state.LocalSize
	if err = runtime.Reallocate(&recovery, uint64(4<<20)+uint64(delta)); err == nil || runtime.State().GlobalSize != -500000 || runtime.State().LocalSize != 0 {
		t.Fatal("native accounting restoration", err, runtime.State())
	}
	fragmentMust(t, runtime.Deallocate(&recovery))
	if cause == nil {
		if levels != 0 || engine.spacer == nil {
			t.Fatal("successful constructor did not publish its context", levels)
		}
		fragmentMust(t, engine.Close())
		fragmentMust(t, engine.Close())
		return true, false
	}
	if !errors.Is(cause, arena.ErrInjectedFailure) || engine.spacer != nil {
		t.Fatal("failed constructor published a context or lost the allocation cause", cause, engine.spacer)
	}
	nativeFailure := errors.Is(cause, arena.ErrBounds)
	var receipt umemory.ConstructionFailure
	hasReceipt := errors.As(cause, &receipt)
	if nativeFailure && !hasReceipt {
		t.Fatal("native constructor cleanup lost its owning receipt", cause)
	}
	message := cause.Error()
	if hasReceipt {
		original := receipt.OriginalCause()
		if receipt.RuntimeOwner() != runtime || !errors.Is(original, arena.ErrInjectedFailure) {
			t.Fatal("constructor receipt lost the original operation or native runtime", original)
		}
		fragmentMust(t, receipt.Cleanup())
		fragmentMust(t, receipt.Cleanup())
		if receipt.CleanupCause() != nil || receipt.OriginalCause() != original || receipt.RuntimeOwner() != runtime {
			t.Fatal("constructor cleanup changed its original cause or runtime")
		}
	}
	after := context.Stats()
	if after.LiveBlocks != before.LiveBlocks || after.References != before.References || after.Entries != before.Entries || manager.Stats().LiveReferences != refs {
		t.Fatal("constructor receipt left a copied parameter, alias or AST owner", before, after, refs, manager.Stats().LiveReferences)
	}
	levels, err = engine.NumLevels(predicate)
	fragmentMust(t, err)
	if levels != 0 || engine.spacer == nil {
		t.Fatal("public constructor retry did not publish a fresh context", levels)
	}
	fragmentMust(t, engine.Close())
	fragmentMust(t, engine.Close())
	if hasReceipt {
		fragmentMust(t, receipt.Cleanup())
		fragmentMust(t, receipt.Cleanup())
	}
	if cause.Error() != message || !errors.Is(cause, arena.ErrInjectedFailure) || (nativeFailure && !errors.Is(cause, arena.ErrBounds)) {
		t.Fatal("retry changed the original constructor failure")
	}
	return false, nativeFailure
}
