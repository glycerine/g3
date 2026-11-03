package fp

// Source: spacer_dl_interface::updt_params destroys its context before
// replacement; a refused Go destruction keeps that exact owner for retry.

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/spacer"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestSpacerPrequeryParameterCleanupContinuesNativeDecrement(t *testing.T) {
	for _, disposal := range []bool{false, true} {
		name := "replacement"
		if disposal {
			name = "close"
		}
		t.Run(name, func(t *testing.T) { testSpacerPrequeryParameterStoppedDecrement(t, disposal) })
	}
}

func testSpacerPrequeryParameterStoppedDecrement(t *testing.T, disposal bool) {
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
			t.Fatal("pre-query cleanup leaked bytes", numeralBudget.Used(), paramsBudget.Used(), symbolBudget.Used())
		}
	}()
	module, err := context.NewValue()
	fragmentMust(t, err)
	name := "spacer-prequery-large-numeral"
	integers := context.NumeralsField.Manager.Z
	scoped, err := unumeral.NewScopedInt(integers)
	fragmentMust(t, err)
	value, err := scoped.Get()
	fragmentMust(t, err)
	digits := make([]uint32, 32768)
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
	prequeryName, err := symbols.Intern("prequery-stopped-decrement")
	fragmentMust(t, err)
	predicate, err := manager.MkFuncDecl(prequeryName, nil, boolean, ast.ASTFuncDeclInfo{})
	fragmentMust(t, err)
	fragmentMust(t, engine.RegisterPredicate(predicate))
	_, err = engine.NumLevels(predicate)
	fragmentMust(t, err)
	prequery := engine.spacer
	if prequery == nil {
		t.Fatal("NumLevels did not construct Spacer")
	}

	a, err := runtime.Allocate(8)
	fragmentMust(t, err)
	b, err := runtime.Allocate(8)
	fragmentMust(t, err)
	delta := int64(-1000000)
	if err = runtime.Reallocate(&a, uint64(16)+uint64(delta)-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("runtime refusal priming", err)
	}
	fragmentMust(t, runtime.Deallocate(&a))
	state := runtime.State()
	delta = math.MinInt64 + int64(1000) - state.GlobalSize + 16
	if err = runtime.Reallocate(&b, uint64(delta)+16-8); !errors.Is(err, arena.ErrBounds) {
		t.Fatal("runtime lower-bound priming", err)
	}
	fragmentMust(t, runtime.Deallocate(&b))
	integerBudget := integers.BudgetField
	integerBudget.FailAfter(0)
	paramsBudget.FailAfter(0)
	if disposal {
		err = engine.Close()
	} else {
		err = engine.SetQueryParameters(nil)
	}
	integerBudget.FailAfter(-1)
	paramsBudget.FailAfter(-1)
	if !errors.Is(err, arena.ErrBounds) {
		t.Fatal("parameter snapshot cleanup did not expose native refusal", err)
	}
	if engine.spacer != prequery {
		t.Fatal("lost refused prequery context")
	}
	// Close visits the engine's parameter snapshot before Spacer; parameter
	// replacement visits Spacer's independent solver-parameter copy first.
	if (engine.queryParamsReceipt.Invocation() != 0) != disposal {
		t.Fatal("first refusal reached the wrong parameter owner")
	}
	if disposal {
		err = engine.Close()
	} else {
		err = engine.SetQueryParameters(nil)
	}
	if !errors.Is(err, arena.ErrBounds) {
		t.Fatal("retry did not reach the independent parameter destruction", err)
	}
	if disposal {
		if engine.spacer != prequery || engine.queryParamsReceipt.Invocation() != 0 || engine.queryParams.Slot != 0 {
			t.Fatal("close did not finish its snapshot before refusing Spacer teardown")
		}
	} else {
		if engine.spacer != nil || engine.queryParamsReceipt.Invocation() == 0 {
			t.Fatal("replacement did not finish Spacer before refusing its snapshot")
		}
	}
	if disposal {
		err = engine.Close()
	} else {
		err = engine.SetQueryParameters(nil)
	}
	fragmentMust(t, err)
	if engine.queryParamsReady || engine.queryParams.Slot != 0 || engine.queryParamsReceipt.Invocation() != 0 {
		t.Fatal("parameter replacement retry did not consume exact receipt")
	}
	if engine.spacer != nil {
		t.Fatal("completed cleanup kept old Spacer context")
	}
	if _, err := prequery.Initialized(); !errors.Is(err, spacer.ErrContextOwner) {
		t.Fatal("cleanup retry left original context open", err)
	}
	if !disposal {
		fragmentMust(t, engine.SetQueryParameters(nil))
		levels, err := engine.NumLevels(predicate)
		fragmentMust(t, err)
		if levels != 0 || engine.spacer == nil || engine.spacer == prequery {
			t.Fatal("parameter replacement did not construct fresh pre-query state", levels)
		}
		falsehood, err := manager.MkFalse()
		fragmentMust(t, err)
		fragmentMust(t, engine.AddCover(1, predicate, falsehood))
		levels, err = engine.NumLevels(predicate)
		fragmentMust(t, err)
		if levels != 2 {
			t.Fatal("replacement context could not accept a pre-query cover", levels)
		}
	}
	fragmentMust(t, engine.Close())
	fragmentMust(t, engine.Close())
}
