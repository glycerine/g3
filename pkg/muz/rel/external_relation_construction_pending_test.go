package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestExternalRelationConstructionCleanupPreservesNestedTempsAndResult(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var owner *externalRelationConstructionOwner
	var childAttempt *externalInterpretedAttempt
	var failure error
	active, callbacks, intact := false, 0, true
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(value memory.Failure) error {
		if active && failure == nil {
			callbacks++
			beforeTemp, beforeChild, beforeResult := owner.temps[0], childAttempt.roots[0], owner.result
			child := finishExternalInterpretedAttempt(childAttempt, nil)
			failure = owner.dispose(child)
			intact = errors.Is(failure, ast.ErrASTPending) && owner.temps[0] == beforeTemp &&
				childAttempt.roots[0] == beforeChild && owner.result == beforeResult
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
	relations, err := NewRelationManager(manager)
	must(err)
	plugin, err := NewExternalRelationPlugin(relations, ExternalRelationCallbacks{})
	must(err)
	result, err := plugin.NewEmpty(nil)
	must(err)
	truth, err := manager.MkTrue()
	must(err)
	temporary, err := manager.NewRef(truth)
	must(err)
	child, err := manager.NewRef(truth)
	must(err)
	owner = &externalRelationConstructionOwner{manager: manager, temps: []ast.ASTRef{temporary}, result: result}
	childAttempt = &externalInterpretedAttempt{manager: manager, roots: []ast.ASTRef{child}}
	number, err := numbers.NewValue()
	must(err)
	must(numbers.SetString(&number, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	active = true
	var invocation ast.ASTArithmeticNumeralInvocation
	_, body := manager.ArithmeticNumeralWithInvocation(&number, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || callbacks != 1 || !intact {
		t.Fatal("native construction cleanup did not preserve exact owners", body, callbacks, intact)
	}
	var receipt memory.ConstructionFailure
	if !errors.As(failure, &receipt) {
		t.Fatal("construction cleanup lost typed retry owner", failure)
	}
	owned, ok := receipt.(*externalRelationConstructionFailure)
	if !ok || owned.child == nil || owned.owner != owner || owned.RuntimeOwner() != runtime || owned.OriginalCause() == nil || owned.CleanupCause() == nil {
		t.Fatal("construction receipt lost nested or local owners", owned)
	}
	message, exposed := receipt.Error(), receipt.Unwrap()
	must(receipt.Cleanup())
	if receipt.Error() != message || receipt.Unwrap() != exposed || !errors.Is(receipt, ast.ErrASTPending) || receipt.CleanupCause() != nil {
		t.Fatal("construction receipt changed stable refusal identity")
	}
	if owned.child != nil || owner.temps[0] != (ast.ASTRef{}) || owner.result != nil || len(childAttempt.roots) != 0 || plugin.users != 0 {
		t.Fatal("construction retry left concrete owners")
	}
	must(receipt.Cleanup())
	must(plugin.Close())
	must(relations.Close())
	must(numbers.Del(&number))
	must(manager.Close())
	must(numbers.Close())
	must(symbols.Close())
	must(runtime.Close())
	if used := budget.Used(); used != 0 {
		t.Fatal("external construction refusal fixture leaked bytes", used)
	}
}

func TestExternalRelationConstructorCallsitesSuppressFailedOutputs(t *testing.T) {
	f := newTableRelationFixture(t)
	callbackFailure := errors.New("external callback refused construction")
	failing := int32(-1)
	callbacks := ExternalRelationCallbacks{
		ReduceAssign: func(declaration ast.ASTHandle, _, _ []ast.ASTHandle) error {
			kind, err := f.ast.GetDeclKind(declaration)
			if err != nil {
				return err
			}
			if kind == failing {
				return callbackFailure
			}
			return nil
		},
		Reduce: func(_ ast.ASTHandle, arguments []ast.ASTHandle) (ast.ASTHandle, error) {
			return arguments[0], nil
		},
	}
	plugin, err := NewExternalRelationPlugin(f.relations, callbacks)
	astTestMust(t, err)

	failing = ast.DatalogOpEmpty
	failedEmpty, err := plugin.NewEmpty(nil)
	if failedEmpty != nil || !errors.Is(err, callbackFailure) || plugin.users != 0 {
		t.Fatal("NewEmpty published or leaked its failed concrete output", failedEmpty, err, plugin.users)
	}

	failing = -1
	receiver, err := plugin.NewEmpty(nil)
	astTestMust(t, err)
	if plugin.users != 1 {
		t.Fatal("receiver ownership was not established", plugin.users)
	}
	failing = ast.DatalogOpClone
	failedClone, err := receiver.Clone()
	if failedClone != nil || !errors.Is(err, callbackFailure) || plugin.users != 1 {
		t.Fatal("Clone published or leaked its failed concrete output", failedClone, err, plugin.users)
	}
	if _, err = receiver.Relation(); err != nil {
		t.Fatal("Clone cleanup consumed its borrowed receiver", err)
	}

	failing = -1
	complement, err := receiver.Complement()
	astTestMust(t, err)
	if plugin.users != 2 {
		t.Fatal("Complement did not own exactly its new output", plugin.users)
	}
	astTestMust(t, complement.Close())
	if plugin.users != 1 {
		t.Fatal("Complement close consumed or leaked receiver ownership", plugin.users)
	}
	if _, err = receiver.Relation(); err != nil {
		t.Fatal("Complement treated its borrowed receiver as owned", err)
	}

	astTestMust(t, receiver.Close())
	astTestMust(t, plugin.Close())
}
