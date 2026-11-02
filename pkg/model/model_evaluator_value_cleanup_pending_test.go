package model

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

var errModelEvaluationInjected = errors.New("model evaluation failed after result allocation")

func TestModelEvaluatorValueFailurePreservesResultOnNativeCleanupRefusal(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var manager *ast.ASTManager
	var result ast.ASTRef
	var failure error
	var runtime *memory.Runtime
	active := false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(source memory.Failure) error {
		if active && failure == nil {
			failure = finishModelEvaluationFailure(manager, runtime, result, errModelEvaluationInjected)
		}
		return source
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
		if budget.Used() != 0 {
			t.Fatal("fixture leaked bytes", budget.Used())
		}
	}()
	_, err = manager.RegisterArithmeticPlugin()
	must(err)
	truth, err := manager.MkTrue()
	must(err)
	result, err = manager.NewRef(truth)
	must(err)
	before, err := manager.GetRefCount(truth)
	must(err)
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
	if body == nil || !errors.Is(failure, errModelEvaluationInjected) || !errors.Is(failure, ast.ErrASTPending) {
		t.Fatal("did not reach dual evaluation/cleanup failure", body, failure)
	}
	var cleanup memory.ConstructionFailure
	if !errors.As(failure, &cleanup) {
		t.Fatal("lost retryable result owner", failure)
	}
	owned, ok := failure.(*modelEvaluationConstructionFailure)
	if !ok || owned.RuntimeOwner() != runtime || owned.OriginalCause() != errModelEvaluationInjected || !errors.Is(owned.CleanupCause(), ast.ErrASTPending) || owned.result != result {
		t.Fatal("incomplete construction owner", owned)
	}
	handle, err := manager.RefGet(owned.result)
	must(err)
	if handle != truth {
		t.Fatal("construction owner kept wrong result", handle, truth)
	}
	count, err := manager.GetRefCount(truth)
	must(err)
	if count != before {
		t.Fatal("refused cleanup changed result ownership", before, count)
	}
	must(cleanup.Cleanup())
	must(cleanup.Cleanup())
	if owned.result != (ast.ASTRef{}) || owned.CleanupCause() != nil {
		t.Fatal("cleanup retry did not retire result owner", owned.result, owned.CleanupCause())
	}
	count, err = manager.GetRefCount(truth)
	must(err)
	if count+1 != before {
		t.Fatal("cleanup retry released wrong reference count", before, count)
	}
}
