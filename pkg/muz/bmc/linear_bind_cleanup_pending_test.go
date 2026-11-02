package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp bmc::linear::mk_rule_vars and
// bmc::linear::get_model binding-vector lifetime across native AST refusal.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestLinearBindingCleanupPreservesInputsAndOutputsAcrossNativeRefusal(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var manager *ast.ASTManager
	var owner *linearBindingOwner
	var failure error
	callbacks, intact := 0, true
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		callbacks++
		beforeInput, beforeOutput := owner.inputs[0], owner.outputs[0]
		failure = owner.dispose(nil)
		intact = intact && errors.Is(failure, ast.ErrASTPending) &&
			owner.inputs[0] == beforeInput && owner.outputs[0] == beforeOutput
		return f
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	manager, err = ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	defer func() {
		must(manager.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if used := budget.Used(); used != 0 {
			t.Fatal("linear binding cleanup fixture leaked bytes", used)
		}
	}()
	_, err = manager.RegisterArithmeticPlugin()
	must(err)
	value, err := manager.MkTrue()
	must(err)
	input, err := manager.NewRef(value)
	must(err)
	output, err := manager.NewRef(value)
	must(err)
	owner = &linearBindingOwner{manager: manager, inputs: []ast.ASTRef{input}, outputs: []ast.ASTRef{output}}

	number, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&number)) }()
	must(numbers.SetString(&number, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	_, body := manager.ArithmeticNumeralWithInvocation(&number, false, &invocation)
	must(runtime.SetMaxCount(0))
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || callbacks == 0 || !intact {
		t.Fatal("refused linear binding cleanup lost an owner", body, callbacks, intact)
	}
	var cleanup memory.ConstructionFailure
	if !errors.As(failure, &cleanup) {
		t.Fatal("linear binding cleanup lost concrete retry owner", failure)
	}
	must(cleanup.Cleanup())
	if cleanup.Error() == "" || !errors.Is(cleanup.Unwrap(), ast.ErrASTPending) {
		t.Fatal("linear binding cleanup lost stable refusal cause")
	}
	if owner.inputs[0] != (ast.ASTRef{}) || owner.outputs[0] != (ast.ASTRef{}) {
		t.Fatal("linear binding cleanup retry left references")
	}
	if err = owner.dispose(nil); err != nil {
		t.Fatal("linear binding cleanup is not idempotent", err)
	}
}
