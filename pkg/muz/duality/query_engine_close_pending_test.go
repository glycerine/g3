package duality

import (
	"errors"
	"math"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestDualityBackendRejectsQueriesWhileCleanupIsPending(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 30)
	var backend *Backend
	active, callback := false, false
	var closeErr error
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active && !callback {
			callback = true
			closeErr = backend.Close()
			if !errors.Is(closeErr, ast.ErrASTPending) || backend.queries == nil || !backend.queries.closing || backend.queries.closed || len(backend.queries.runs) != 1 {
				t.Fatalf("refused cleanup lost its concrete query owner: %v", closeErr)
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
	cooperation, err := cooperate.NewCooperation(budget)
	must(err)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	must(manager.RegisterDeclarationPlugins())
	config := smtcore.SMTSolverConfig{Params: params, Globals: globals, ModelEvaluator: model.ModelEvaluatorConfig{Context: params, Globals: globals, Memory: runtime, Cooperation: cooperation}}
	backend, err = NewBackend(manager, config, true)
	must(err)
	defer func() {
		active = false
		must(runtime.SetMaxCount(0))
		if backend != nil {
			must(backend.Close())
		}
		must(manager.Close())
		must(cooperation.Close())
		must(globals.Close())
		must(catalog.Close())
		must(params.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("Duality cleanup fixture leaked bytes", budget.Used())
		}
	}()

	query, err := manager.MkFalse()
	must(err)
	prepared, err := PrepareQuery(manager, nil, nil, query, nil, math.MaxUint32)
	must(err)
	defer func() { must(prepared.Close()) }()
	status, err := backend.Query(prepared)
	must(err)
	if status != lbool.False || backend.queries.active == nil {
		t.Fatal("fixture did not publish a real safe query", status)
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
		t.Fatal("numerical callback did not refuse Duality cleanup", body, callback, closeErr)
	}
	if status, err = backend.Query(prepared); status != lbool.Undef || err == nil {
		t.Fatal("query entered cleanup-pending backend", status, err)
	}
	if md, err := backend.GetModel(); md != nil || err == nil {
		t.Fatal("model escaped cleanup-pending backend", md, err)
	}
	if backend.Counterexample() != nil {
		t.Fatal("counterexample escaped cleanup-pending backend")
	}
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	must(backend.Close())
	must(backend.Close())
	if !backend.closed || backend.queries != nil {
		t.Fatal("cleanup retry did not retire backend")
	}
}
