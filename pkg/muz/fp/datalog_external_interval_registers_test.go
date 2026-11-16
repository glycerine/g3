package fp

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestExternalIntervalDatalogRegisterStorePublishesBeforeRetirementRetry(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var registers *externalIntervalDatalogRegisters
	var storeFailure error
	active, fired := false, false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(failure memory.Failure) error {
		if active && !fired {
			fired = true
			storeFailure = registers.store()
		}
		return failure
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
	must(manager.RegisterDeclarationPlugins())
	family, err := manager.RegisterArithmeticPlugin()
	must(err)
	integer, err := manager.MkFamilySort(family, 1, nil)
	must(err)
	boolean, err := manager.MkBoolSort()
	must(err)
	name, err := symbols.Intern("external-interval-register-retirement")
	must(err)
	predicate, err := manager.MkFuncDecl(name, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	relations, err := rel.NewRelationManager(manager)
	must(err)
	callbacks := &externalIntervalCallbackState{manager: manager, nonempty: make(map[ast.ASTHandle]bool)}
	plugin, err := rel.NewExternalRelationPlugin(relations, callbacks.callbacks())
	must(err)
	activation, err := newExternalIntervalDatalogActivation(manager, plugin, []externalIntervalDatalogPredicate{{
		declaration: predicate,
		sorts:       []ast.ASTHandle{integer},
		kinds:       []string{"external_relation", "interval_relation"},
	}})
	must(err)
	value, err := manager.ArithmeticIntegerNumeralText("13")
	must(err)
	must(activation.addFact(predicate, []ast.ASTHandle{value}))
	old := activation.entries[predicate].product
	registers, err = activation.loadExternalIntervalDatalogRegisters([]ast.ASTHandle{predicate})
	must(err)
	loaded, err := registers.relation(predicate)
	must(err)
	if loaded == nil || loaded == old {
		t.Fatal("retirement fixture did not load an independent clone")
	}
	callbacksBeforeStore := len(callbacks.operations)

	number, err := numbers.NewValue()
	must(err)
	must(numbers.SetString(&number, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := manager.ArithmeticNumeralWithInvocation(&number, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || !fired || storeFailure == nil {
		t.Fatal("native retirement fixture did not invoke STORE", body, fired, storeFailure)
	}
	var receipt memory.ConstructionFailure
	if !errors.As(storeFailure, &receipt) || receipt.RuntimeOwner() != runtime || !errors.Is(receipt.CleanupCause(), ast.ErrASTPending) {
		t.Fatal("STORE retirement refusal lost typed retry owner", storeFailure)
	}
	if activation.entries[predicate].product != loaded || len(callbacks.operations) != callbacksBeforeStore {
		t.Fatal("retirement refusal failed to publish once or replayed a callback", callbacks.operations)
	}
	empty, err := activation.entries[predicate].product.Empty()
	must(err)
	if empty {
		t.Fatal("published replacement lost the cloned nonempty state")
	}
	callbacksAfterProbe := len(callbacks.operations)
	shown, exposed, original := receipt.Error(), receipt.Unwrap(), receipt.OriginalCause()
	must(receipt.Cleanup())
	must(receipt.Cleanup())
	if receipt.Error() != shown || receipt.Unwrap() != exposed || receipt.OriginalCause() != original || receipt.CleanupCause() != nil || !registers.closed || len(callbacks.operations) != callbacksAfterProbe {
		t.Fatal("retirement retry changed receipt identity or left register owner open")
	}
	if _, err = old.Empty(); !errors.Is(err, rel.ErrTableRelation) {
		t.Fatal("retirement retry left the displaced product usable", err)
	}

	must(numbers.Del(&number))
	must(activation.close())
	must(plugin.Close())
	must(relations.Close())
	must(manager.Close())
	must(numbers.Close())
	must(symbols.Close())
	must(runtime.Close())
	if used := budget.Used(); used != 0 {
		t.Fatal("external interval register retirement fixture leaked bytes", used)
	}
}

func assertExternalIntervalRegisterOperations(t *testing.T, got, want []int32) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatal("external register callback count", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatal("external register callback order", got, want)
		}
	}
}
