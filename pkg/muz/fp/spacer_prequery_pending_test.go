package fp

// Source: spacer_dl_interface::updt_params replaces the owned context.
// Go native AST refusal must leave every pre-query child available for retry.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/spacer"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestSpacerPrequeryMutationPreservesNativePendingOwner(t *testing.T) {
	for _, local := range []bool{true, false} {
		name := "module-update"
		if local {
			name = "local-update"
		}
		t.Run(name, func(t *testing.T) { testSpacerPrequeryNativePending(t, local) })
	}
}

func testSpacerPrequeryNativePending(t *testing.T, local bool) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var engine *PropositionalEngine
	var original *spacer.Context
	var revision uint64
	mutate := func() error {
		if local {
			return engine.SetQueryParameters(nil)
		}
		return engine.ParametersUpdated()
	}
	var cleanupErrors []error
	preserved := true
	var runtime *memory.Runtime
	var err error
	active, callback := false, false
	var closeErr error
	runtime, err = memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active && !callback {
			callback = true
			for i := 0; i < 2; i++ {
				closeErr = mutate()
				cleanupErrors = append(cleanupErrors, closeErr)
				preserved = preserved && errors.Is(closeErr, ast.ErrASTPending) && engine.spacer == original && engine.spacerRevision == revision
			}
		}
		return f
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
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
	defer func() {
		active = false
		must(runtime.SetMaxCount(0))
		if engine != nil {
			must(engine.Close())
		}

		must(manager.Close())
		must(globals.Close())
		must(catalog.Close())
		must(params.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("Spacer pre-query pending fixture leaked bytes", budget.Used())
		}
	}()
	engine, err = NewPropositionalEngineWithServices(manager, params, globals)
	must(err)
	must(engine.SelectEngine("spacer"))
	boolean, err := manager.MkBoolSort()
	must(err)
	name, err := symbols.Intern("pending-spacer-prequery")
	must(err)
	predicate, err := manager.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(engine.RegisterPredicate(predicate))
	_, err = engine.NumLevels(predicate)
	must(err)
	original = engine.spacer
	revision = engine.spacerRevision
	beforeParameters, beforeReferences := params.Stats(), manager.Stats().LiveReferences
	if original == nil {
		t.Fatal("public NumLevels did not install Spacer")
	}
	value, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&value)) }()
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	if body == nil || !callback || !errors.Is(closeErr, ast.ErrASTPending) {
		t.Fatal("numerical callback did not refuse Spacer cleanup", body, callback, closeErr)
	}
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	if !preserved || len(cleanupErrors) != 2 {
		t.Fatal("pending public replacement lost exact prequery owner", cleanupErrors)
	}
	for _, cause := range cleanupErrors {
		if !errors.Is(cause, ast.ErrASTPending) || errors.Is(cause, spacer.ErrSymMuxOwner) {
			t.Fatal("native refusal prematurely destroyed a sibling", cause)
		}
	}
	afterParameters := params.Stats()
	if afterParameters.LiveBlocks != beforeParameters.LiveBlocks || afterParameters.References != beforeParameters.References || afterParameters.Entries != beforeParameters.Entries || manager.Stats().LiveReferences != beforeReferences {
		t.Fatal("native refusal changed pre-query ownership", beforeParameters, afterParameters, beforeReferences, manager.Stats().LiveReferences)
	}
	err = mutate()
	if err != nil {
		t.Fatalf("public replacement retry failed: %v", err)
	}
	if engine.spacer != nil {
		t.Fatal("public retry kept disposed Spacer owner")
	}
	if _, err := original.Initialized(); !errors.Is(err, spacer.ErrContextOwner) {
		t.Fatal("successful replacement left original context open", err)
	}
	must(mutate())
	_, err = engine.NumLevels(predicate)
	must(err)
	if engine.spacer == nil || engine.spacer == original {
		t.Fatal("public retry did not construct fresh context")
	}
	falsehood, err := manager.MkFalse()
	must(err)
	must(engine.AddCover(1, predicate, falsehood))
	levels, err := engine.NumLevels(predicate)
	must(err)
	if levels != 2 {
		t.Fatal("replacement context did not accept a fresh cover", levels)
	}
	must(engine.Close())
	must(engine.Close())
}
