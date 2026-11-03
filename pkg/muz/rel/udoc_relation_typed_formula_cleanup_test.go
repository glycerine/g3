package rel

import (
	"errors"
	"math/big"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestTypedUDocFormulaNativeCleanupRefusalKeepsUnpublishedResult(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var owner *typedUDocFormulaOwner
	var failure error
	active, refusing := false, false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(value memory.Failure) error {
		if active && !refusing && failure == nil {
			refusing = true
			failure = finishTypedUDocFormula(owner, nil)
			refusing = false
		}
		return value
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{
		Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true},
		Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}},
	}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	manager, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	_, err = manager.RegisterArithmeticPlugin()
	must(err)
	_, err = manager.RegisterBitVectorPlugin()
	must(err)
	guard, err := numbers.NewValue()
	must(err)
	must(numbers.SetString(&guard, "18446744073709551617/18446744073709551619"))
	_, err = manager.ArithmeticNumeral(&guard, false)
	must(err)

	// Reach the actual ToFormula numeral path, including its eager numerical
	// cleanup. Only its expr_ref frontier remains when finalization starts.
	owner = &typedUDocFormulaOwner{manager: manager}
	sort, err := manager.BitVectorSort(160)
	must(err)
	variable, err := owner.hold(manager.MkVar(0, sort))
	must(err)
	value, err := owner.bitVectorNumeral(big.NewInt(5), 160)
	must(err)
	equal, err := owner.hold(manager.MkEq(variable, value))
	must(err)
	owner.result, err = manager.NewRef(equal)
	must(err)
	if len(owner.numbers) != 0 || len(owner.calls) != 0 {
		t.Fatal("completed numeral left a numerical scope pending")
	}
	resultRef := owner.result
	temporaryRefs := append([]ast.ASTRef(nil), owner.roots...)

	// An already-consumed trailing root tests the reverse, zero-aware frontier.
	consumed, err := manager.NewRef(equal)
	must(err)
	must(manager.DelRef(&consumed))
	owner.roots = append(owner.roots, consumed)
	liveHandle, err := manager.MkTrue()
	must(err)
	liveRef, err := manager.NewRef(liveHandle)
	must(err)

	// A native arithmetic-factory guard makes DelRef refuse with ASTPending.
	// Reenter only the expr_ref finalizer: numeral cleanup is already complete,
	// so it does not try to reacquire the executing rational-service mutex.
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	active = true
	var invocation ast.ASTArithmeticNumeralInvocation
	_, body := manager.ArithmeticNumeralWithInvocation(&guard, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || !errors.Is(failure, ast.ErrASTPending) {
		t.Fatal("native formula finalizer refusal was not reached", body, failure)
	}
	var receipt memory.ConstructionFailure
	if !errors.As(failure, &receipt) {
		t.Fatal("formula cleanup refusal lost its retry owner", failure)
	}
	owned, ok := receipt.(*typedUDocFormulaCleanupFailure)
	if !ok || owned.owner != owner || receipt.RuntimeOwner() != runtime || receipt.OriginalCause() == nil || receipt.CleanupCause() == nil {
		t.Fatal("formula cleanup receipt lost concrete owner or causes", receipt)
	}
	if len(owner.roots) != len(temporaryRefs) || owner.result != resultRef {
		t.Fatal("refused temporary cleanup retired the result or lost its reverse frontier", len(owner.roots), owner.result)
	}
	for index, ref := range temporaryRefs {
		if owner.roots[index] != ref {
			t.Fatal("native refusal consumed an earlier temporary", index)
		}
		_, err = manager.RefGet(ref)
		must(err)
	}
	got, err := manager.RefGet(owner.result)
	must(err)
	if got != equal {
		t.Fatal("unpublished formula changed during refused cleanup", got, equal)
	}

	message, exposed := receipt.Error(), receipt.Unwrap()
	must(receipt.Cleanup())
	if receipt.Error() != message || receipt.Unwrap() != exposed || !errors.Is(receipt, ast.ErrASTPending) || receipt.CleanupCause() != nil {
		t.Fatal("successful formula cleanup changed its stable error")
	}
	if len(owner.roots) != 0 || owner.result != (ast.ASTRef{}) || len(owner.numbers) != 0 || len(owner.calls) != 0 {
		t.Fatal("formula cleanup retry left an unpublished owner")
	}
	if _, err = manager.RefGet(resultRef); err == nil {
		t.Fatal("cleanup retry did not dispose the unpublished result")
	}
	for _, ref := range temporaryRefs {
		if _, err = manager.RefGet(ref); err == nil {
			t.Fatal("cleanup retry left a temporary root live")
		}
	}
	must(receipt.Cleanup())
	got, err = manager.RefGet(liveRef)
	must(err)
	if got != liveHandle {
		t.Fatal("formula cleanup invalidated an unrelated live root")
	}
	must(manager.DelRef(&liveRef))
	must(numbers.Del(&guard))
	must(manager.Close())
	must(numbers.Close())
	must(symbols.Close())
	must(runtime.Close())
	if used := budget.Used(); used != 0 {
		t.Fatal("formula cleanup fixture leaked bytes", used)
	}
}
