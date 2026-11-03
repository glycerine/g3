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

func TestImplicantCleanupResultConsumedErrorAdvancesFrontier(t *testing.T) {
	manager, _ := newSpacerTestServices(t)
	scope := &implicantCleanupFailure{manager: manager, result: []ast.ASTRef{{}}}
	if err := scope.Cleanup(); !errors.Is(err, ast.ErrASTHandle) || len(scope.result) != 0 {
		t.Fatal("consumed result slot remained in the retry frontier", err, scope.result)
	}
	if err := scope.Cleanup(); err != nil {
		t.Fatal("retry revisited a consumed result slot", err)
	}
}

func TestImplicantCleanupReceiptRetriesNativeRefusal(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 26)
	var successScope *implicantCleanupFailure
	var resultScope *implicantCleanupFailure
	var successPublished ast.ASTRef
	var resultPublished ast.ASTRef
	var successFinalization error
	var resultFinalization error
	active, phase, callbacks, intact := false, 0, 0, true
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
		if active {
			active = false
			callbacks++
			if phase == 1 {
				output, body := finishImplicantSuccess(successScope, []ast.ASTRef{successPublished})
				successFinalization = body
				intact = intact && errors.Is(body, ast.ErrASTPending) && len(output) == 0 &&
					len(successScope.temporary) == 1 && len(successScope.result) == 1 && successScope.result[0] == successPublished
			} else {
				body := resultScope.Cleanup()
				resultFinalization = resultScope
				intact = intact && errors.Is(body, ast.ErrASTPending) &&
					len(resultScope.result) == 1 && resultScope.result[0] == resultPublished
			}
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
	params, err := uparams.NewParamsContext(symbols, numerals, uparams.ParamsConfig{Budget: budget})
	must(err)
	catalog, err := uparams.NewGlobalParamsCatalog(symbols, uparams.GlobalParamsCatalogConfig{Budget: budget})
	must(err)
	globals, err := uparams.NewGlobalParamsService(symbols, params, uparams.GlobalParamsConfig{Budget: budget, Catalog: catalog})
	must(err)
	must(globals.InitializeSource())
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numerals, Budget: budget})
	must(err)
	must(manager.RegisterDeclarationPlugins())
	defer func() {
		active = false
		must(runtime.SetMaxCount(0))
		if successScope != nil {
			must(successScope.Cleanup())
		}
		if resultScope != nil {
			must(resultScope.Cleanup())
		}
		must(manager.Close())
		must(globals.Close())
		must(catalog.Close())
		must(params.Close())
		must(numerals.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("implicant cleanup fixture leaked bytes", budget.Used())
		}
	}()
	truth, err := manager.MkTrue()
	must(err)
	temporary, err := manager.NewRef(truth)
	must(err)
	successPublished, err = manager.NewRef(truth)
	must(err)
	resultPublished, err = manager.NewRef(truth)
	must(err)
	successScope = &implicantCleanupFailure{manager: manager, temporary: []ast.ASTRef{temporary}, runtime: runtime}
	resultScope = &implicantCleanupFailure{manager: manager, result: []ast.ASTRef{resultPublished}, runtime: runtime, cause: errors.New("result cleanup origin")}
	value, err := numerals.NewValue()
	must(err)
	defer func() { must(numerals.Del(&value)) }()
	must(numerals.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	phase, active = 1, true
	_, body := manager.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	if body == nil || callbacks != 1 || !intact {
		t.Fatal("native refusal did not transfer the success result", body, callbacks, intact, successFinalization, successScope.result)
	}
	var successReceipt memory.ConstructionFailure
	if !errors.As(successFinalization, &successReceipt) || successReceipt.RuntimeOwner() != runtime {
		t.Fatal("success finalizer did not publish its cleanup receipt", successFinalization)
	}
	successOrigin := successReceipt.OriginalCause()
	successMessage, successExposed := successReceipt.Error(), successReceipt.Unwrap()
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	must(successReceipt.Cleanup())
	must(successReceipt.Cleanup())
	if successReceipt.CleanupCause() != nil || successReceipt.OriginalCause() != successOrigin || !errors.Is(successReceipt, successOrigin) || successReceipt.Error() != successMessage || successReceipt.Unwrap() != successExposed || len(successScope.result) != 0 {
		t.Fatal("success-finalizer cleanup retry lost its receipt", successReceipt.CleanupCause(), successScope.result)
	}
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var resultInvocation ast.ASTArithmeticNumeralInvocation
	phase, active = 2, true
	_, resultBody := manager.ArithmeticNumeralWithInvocation(&value, false, &resultInvocation)
	active = false
	must(runtime.SetMaxCount(0))
	if resultBody == nil || callbacks != 2 || !intact {
		t.Fatal("native refusal did not preserve the result owner", resultBody, callbacks, intact, resultFinalization, resultScope.result)
	}
	var resultReceipt memory.ConstructionFailure
	if !errors.As(resultFinalization, &resultReceipt) || resultReceipt.RuntimeOwner() != runtime {
		t.Fatal("result-stage finalization did not publish its cleanup receipt", resultFinalization)
	}
	origin := resultReceipt.OriginalCause()
	message, exposed := resultReceipt.Error(), resultReceipt.Unwrap()
	if origin == nil || resultReceipt.CleanupCause() == nil || resultFinalization.Error() == "" {
		t.Fatal("cleanup receipt lost the result-stage refusal", resultFinalization)
	}
	must(manager.CleanupArithmeticNumeralInvocation(&resultInvocation))
	must(resultReceipt.Cleanup())
	must(resultReceipt.Cleanup())
	if resultReceipt.CleanupCause() != nil || resultReceipt.OriginalCause() != origin || !errors.Is(resultReceipt, origin) || len(resultScope.result) != 0 || resultReceipt.Error() != message || resultReceipt.Unwrap() != exposed {
		t.Fatal("result-stage cleanup retry lost its frontier", resultReceipt.CleanupCause(), resultScope.result)
	}
}
