package spacer

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestSimplifyBoundsPreservesConsumedInputsWhenCleanupIsPending(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 26)
	var manager *ast.ASTManager
	var params *uparams.ParamsContext
	var globals *uparams.GlobalParamsService
	var inputs []ast.ASTRef
	var failure error
	active, callbacks := false, 0
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
		if active {
			callbacks++
			_, failure = simplifyBounds(manager, params, globals, inputs)
		}
		return source
	}})
	must(err)
	numerals, err := unumeral.NewRationalService(unumeral.RationalServiceConfig{Integers: unumeral.IntegerConfig{
		Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true},
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
	must(err)
	symbols, err := usymbol.NewManager(usymbol.Config{Budget: budget})
	must(err)
	params, err = uparams.NewParamsContext(symbols, numerals, uparams.ParamsConfig{Budget: budget})
	must(err)
	catalog, err := uparams.NewGlobalParamsCatalog(symbols, uparams.GlobalParamsCatalogConfig{Budget: budget})
	must(err)
	globals, err = uparams.NewGlobalParamsService(symbols, params, uparams.GlobalParamsConfig{Budget: budget, Catalog: catalog})
	must(err)
	must(globals.InitializeSource())
	manager, err = ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: budget})
	must(err)
	defer func() {
		active = false
		must(runtime.SetMaxCount(0))
		if cleanup, ok := failure.(memory.ConstructionFailure); ok {
			must(cleanup.Cleanup())
		}
		must(closeSimplifyBoundRefs(manager, inputs))
		must(manager.Close())
		must(globals.Close())
		must(catalog.Close())
		must(params.Close())
		must(numerals.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("simplify-bounds pending fixture leaked bytes", budget.Used())
		}
	}()
	must(manager.RegisterDeclarationPlugins())
	zero, err := manager.ArithmeticIntegerNumeralText("0")
	must(err)
	one, err := manager.ArithmeticIntegerNumeralText("1")
	must(err)
	bound, err := manager.MkArithmeticLE(zero, one)
	must(err)
	input, err := manager.NewRef(bound)
	must(err)
	inputs = []ast.ASTRef{input}
	value, err := numerals.NewValue()
	must(err)
	defer func() { must(numerals.Del(&value)) }()
	must(numerals.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	var cleanup memory.ConstructionFailure
	if body == nil || callbacks == 0 || !errors.As(failure, &cleanup) || !errors.Is(cleanup.CleanupCause(), ast.ErrASTPending) {
		t.Fatal("pending input cleanup was not published", body, callbacks, failure)
	}
	if inputs[0] == (ast.ASTRef{}) || cleanup.RuntimeOwner() != runtime {
		t.Fatal("cleanup receipt lost the transferred input owner")
	}
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	must(cleanup.Cleanup())
	must(cleanup.Cleanup())
	if inputs[0] != (ast.ASTRef{}) || cleanup.CleanupCause() != nil {
		t.Fatal("cleanup retry did not consume input", inputs[0], cleanup.CleanupCause())
	}
	failure = nil
}
