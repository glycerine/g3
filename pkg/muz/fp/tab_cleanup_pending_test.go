package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestTABBackendAttemptPreservesRefusedCleanup(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 29)
	var attempt *tabBackendAttempt
	var failure error
	active, callback := false, false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active && !callback {
			callback = true
			failure = attempt.dispose()
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
	root, err := NewPropositionalEngineWithServices(manager, params, globals)
	must(err)
	defer func() {
		active = false
		must(runtime.SetMaxCount(0))
		if attempt != nil {
			must(attempt.close())
		}
		must(root.Close())
		must(manager.Close())
		must(globals.Close())
		must(catalog.Close())
		must(params.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("TAB cleanup fixture leaked bytes", budget.Used())
		}
	}()

	attempt = &tabBackendAttempt{manager: manager, params: params}
	attempt.view, err = root.fixedpointParamView()
	must(err)
	attempt.rw, err = rewriter.NewTheoryRewriter(manager, params, globals, nil)
	must(err)
	attempt.coop, err = cooperate.NewCooperation(budget)
	must(err)
	attempt.lite, err = qe.NewLite(manager, nil, false, qe.Config{Context: params, Globals: globals, Cooperation: attempt.coop})
	must(err)
	truth, err := manager.MkTrue()
	must(err)
	attempt.queryRef, err = manager.NewRef(truth)
	must(err)
	originalRef := attempt.queryRef

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
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || !callback || !errors.Is(failure, ast.ErrASTPending) {
		t.Fatal("expected actual TAB backend cleanup refusal", body, callback, failure)
	}
	if attempt.view == nil || attempt.rw == nil || attempt.coop == nil || attempt.lite == nil || attempt.queryRef != originalRef {
		t.Fatal("refused TAB cleanup lost or advanced an owner")
	}
	var cleanup memory.ConstructionFailure
	if !errors.As(failure, &cleanup) || cleanup.RuntimeOwner() != runtime || cleanup.OriginalCause() == nil || cleanup.CleanupCause() == nil {
		t.Fatal("TAB backend cleanup refusal is not retryable", failure)
	}
	must(cleanup.Cleanup())
	if attempt.view != nil || attempt.rw != nil || attempt.coop != nil || attempt.lite != nil || attempt.queryRef != (ast.ASTRef{}) || cleanup.CleanupCause() != nil {
		t.Fatal("TAB backend cleanup retry did not retire all owners")
	}
}
