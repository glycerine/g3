package spacer

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func TestArrayGeneralizerCleanupPreservesNativeOperationAndSuccessFrontiers(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 27)
	operationCause := errors.New("array generalizer operation failed")
	var operationAttempt, successAttempt *arrayGeneralizerAttempt
	var operationFailure, successFailure error
	active, phase, callbacks, intact := false, 0, 0, true
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
		if active {
			callbacks++
			if phase == 1 {
				operationFailure = finishArrayGeneralizerAttempt(operationAttempt, operationCause)
				intact = intact && len(operationAttempt.candidates) == 1 && len(operationAttempt.core) == 1 && len(operationAttempt.output) == 1 && operationAttempt.checker != nil
			} else {
				successFailure = finishArrayGeneralizerAttempt(successAttempt, nil)
				intact = intact && len(successAttempt.candidates) == 1 && len(successAttempt.core) == 1 && len(successAttempt.output) == 1 && successAttempt.checker != nil
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
	spacerManager, err := NewManager(manager, nil, 1, smtcore.SMTSolverConfig{Params: params, Globals: globals})
	must(err)
	defer func() {
		active = false
		must(runtime.SetMaxCount(0))
		if receipt, ok := operationFailure.(memory.ConstructionFailure); ok {
			must(receipt.Cleanup())
		}
		if receipt, ok := successFailure.(memory.ConstructionFailure); ok {
			must(receipt.Cleanup())
		}
		must(spacerManager.Close())
		must(manager.Close())
		must(globals.Close())
		must(catalog.Close())
		must(params.Close())
		must(numerals.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("array generalizer cleanup fixture leaked bytes", budget.Used())
		}
	}()
	truth, err := manager.MkTrue()
	must(err)
	newAttempt := func() *arrayGeneralizerAttempt {
		family := spacerManager.contexts[1]
		checker, cause := smtcore.NewSMTSolver(manager, family.params, "", family.config)
		must(cause)
		must(checker.Assert(truth))
		candidate, cause := manager.NewRef(truth)
		must(cause)
		core, cause := manager.NewRef(truth)
		must(cause)
		output, cause := manager.NewRef(truth)
		must(cause)
		return &arrayGeneralizerAttempt{manager: manager, checker: checker,
			candidates: []ast.ASTRef{candidate}, core: []ast.ASTRef{core}, output: []ast.ASTRef{output}}
	}
	operationAttempt, successAttempt = newAttempt(), newAttempt()
	value, err := numerals.NewValue()
	must(err)
	defer func() { must(numerals.Del(&value)) }()
	must(numerals.SetString(&value, "18446744073709551617/18446744073709551619"))
	trigger := func(currentPhase int, invocation *ast.ASTArithmeticNumeralInvocation) {
		must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
		phase, active = currentPhase, true
		_, body := manager.ArithmeticNumeralWithInvocation(&value, false, invocation)
		active = false
		must(runtime.SetMaxCount(0))
		if body == nil {
			t.Fatal("native pending trigger did not allocate numeral body")
		}
	}
	var operationInvocation ast.ASTArithmeticNumeralInvocation
	trigger(1, &operationInvocation)
	var operationReceipt memory.ConstructionFailure
	if callbacks != 1 || !intact || !errors.As(operationFailure, &operationReceipt) || operationReceipt.RuntimeOwner() != runtime {
		t.Fatal("operation cleanup refusal lost array generalizer owners", callbacks, intact, operationFailure)
	}
	operationOrigin, operationMessage, operationExposed := operationReceipt.OriginalCause(), operationReceipt.Error(), operationReceipt.Unwrap()
	if operationOrigin != operationCause || !errors.Is(operationFailure, operationCause) {
		t.Fatal("operation cleanup receipt lost semantic origin")
	}
	must(manager.CleanupArithmeticNumeralInvocation(&operationInvocation))
	must(operationReceipt.Cleanup())
	must(operationReceipt.Cleanup())
	if operationReceipt.CleanupCause() != nil || operationReceipt.OriginalCause() != operationOrigin || operationReceipt.Error() != operationMessage || operationReceipt.Unwrap() != operationExposed || operationAttempt.checker != nil || len(operationAttempt.candidates)+len(operationAttempt.core)+len(operationAttempt.output) != 0 {
		t.Fatal("operation cleanup retry changed receipt or left array owners")
	}
	var successInvocation ast.ASTArithmeticNumeralInvocation
	trigger(2, &successInvocation)
	var successReceipt memory.ConstructionFailure
	if callbacks != 2 || !intact || !errors.As(successFailure, &successReceipt) || successReceipt.RuntimeOwner() != runtime {
		t.Fatal("success cleanup refusal published output or lost owner", callbacks, intact, successFailure)
	}
	successOrigin, successMessage, successExposed := successReceipt.OriginalCause(), successReceipt.Error(), successReceipt.Unwrap()
	if successOrigin == nil || !errors.Is(successFailure, successOrigin) {
		t.Fatal("success cleanup receipt lost cleanup origin")
	}
	must(manager.CleanupArithmeticNumeralInvocation(&successInvocation))
	must(successReceipt.Cleanup())
	must(successReceipt.Cleanup())
	if successReceipt.CleanupCause() != nil || successReceipt.OriginalCause() != successOrigin || successReceipt.Error() != successMessage || successReceipt.Unwrap() != successExposed || successAttempt.checker != nil || len(successAttempt.candidates)+len(successAttempt.core)+len(successAttempt.output) != 0 {
		t.Fatal("success cleanup retry changed receipt or left unpublished output")
	}
	operationFailure, successFailure = nil, nil
}
