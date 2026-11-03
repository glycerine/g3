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

func TestExternalIntervalDatalogGroundSingletonCloseAfterPublicationRetry(t *testing.T) {
	testExternalIntervalDatalogGroundTemporaryCloseAfterPublicationRetry(t, false)
}

func TestExternalIntervalDatalogNullaryTotalCloseAfterPublicationRetry(t *testing.T) {
	testExternalIntervalDatalogGroundTemporaryCloseAfterPublicationRetry(t, true)
}

func testExternalIntervalDatalogGroundTemporaryCloseAfterPublicationRetry(t *testing.T, nullary bool) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var continuation *externalIntervalDatalogGroundContinuation
	var closeFailure error
	active, fired := false, false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(failure memory.Failure) error {
		if active && !fired {
			fired = true
			closeFailure = continuation.resume()
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
	label := "singleton"
	var sorts []ast.ASTHandle
	if nullary {
		label = "nullary-total"
	} else {
		sorts = []ast.ASTHandle{integer}
	}
	name, err := symbols.Intern("external-interval-ground-" + label + "-close")
	must(err)
	predicate, err := manager.MkFuncDecl(name, sorts, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	relations, err := rel.NewRelationManager(manager)
	must(err)
	callbacks := &externalIntervalCallbackState{manager: manager, nonempty: make(map[ast.ASTHandle]bool)}
	plugin, err := rel.NewExternalRelationPlugin(relations, callbacks.callbacks())
	must(err)
	activation, err := newExternalIntervalDatalogActivation(manager, plugin, []externalIntervalDatalogPredicate{{
		declaration: predicate,
		sorts:       sorts,
		kinds:       []string{"external_relation", "interval_relation"},
	}})
	must(err)
	var value ast.ASTHandle
	if !nullary {
		value, err = manager.ArithmeticIntegerNumeralText("17")
		must(err)
	}
	var values []ast.ASTHandle
	if !nullary {
		values = []ast.ASTHandle{value}
	}
	application, err := manager.MkApp(predicate, values)
	must(err)
	must(activation.installExternalIntervalRulePlans(nil, &externalIntervalDatalogGroundRule{head: predicate, application: application, values: values}, nil))

	old := activation.entries[predicate].product
	registers, err := activation.loadExternalIntervalDatalogRegisters([]ast.ASTHandle{predicate})
	must(err)
	var temporary *rel.ExternalIntervalProduct
	var selected bool
	if nullary {
		temporary, selected, err = rel.NewSelectedExternalIntervalNullaryTotal(plugin, []string{"external_relation", "interval_relation"})
	} else {
		temporary, selected, err = rel.NewSelectedExternalIntervalProduct(plugin, sorts, []string{"external_relation", "interval_relation"})
	}
	must(err)
	if !selected || temporary == nil {
		t.Fatal("ground temporary product was not selected")
	}
	if !nullary {
		must(temporary.AddFact([]ast.ASTHandle{value}))
	}
	target, err := registers.ensureRelation(predicate)
	must(err)
	must(target.Union(temporary, nil, false))
	must(registers.store())
	published := activation.entries[predicate].product
	if published == old {
		t.Fatal("ground STORE did not publish the union target")
	}
	if _, err = old.Empty(); !errors.Is(err, rel.ErrTableRelation) {
		t.Fatal("ground STORE did not retire the displaced product", err)
	}
	// Keep a harmless already-published register owner in the continuation.
	// Its STORE continuation must finish before the real temporary Close is
	// attempted, making this fixture sensitive to the source teardown order.
	storeFrontier := &externalIntervalDatalogRegisters{
		activation: activation,
		runtime:    runtime,
		indices:    make(map[ast.ASTHandle]int),
		published:  true,
		retireNext: -1,
	}
	continuation = &externalIntervalDatalogGroundContinuation{
		registers: storeFrontier,
		temporary: temporary,
		runtime:   runtime,
		publish:   true,
	}
	callbacksBeforeClose := len(callbacks.operations)

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
	if body == nil || !fired || !errors.Is(closeFailure, ast.ErrASTPending) || continuation.registers != nil || !storeFrontier.closed || continuation.temporary != temporary {
		t.Fatal("native ground temporary close refusal was not preserved", body, fired, closeFailure)
	}
	if activation.entries[predicate].product != published || len(callbacks.operations) != callbacksBeforeClose {
		t.Fatal("ground temporary close refusal changed publication or replayed callbacks", callbacks.operations)
	}

	pending := newExternalIntervalDatalogGroundFailure(closeFailure, continuation)
	activation.pending = pending
	activation.pendingCompletesRule = true
	activation.pendingGround = activation.ground
	var receipt memory.ConstructionFailure
	if !errors.As(pending, &receipt) || receipt.RuntimeOwner() != runtime || !errors.Is(receipt.CleanupCause(), ast.ErrASTPending) {
		t.Fatal("ground temporary close refusal lost typed continuation", pending)
	}
	shown, exposed, original := receipt.Error(), receipt.Unwrap(), receipt.OriginalCause()
	must(activation.executeExternalIntervalIdentity())
	if activation.pending != nil || activation.pendingCompletesRule || activation.pendingGround != nil || continuation.temporary != nil {
		t.Fatal("ground continuation retry did not finish temporary close")
	}
	if receipt.Error() != shown || receipt.Unwrap() != exposed || receipt.OriginalCause() != original || receipt.CleanupCause() != nil {
		t.Fatal("ground continuation retry changed receipt identity")
	}
	if activation.entries[predicate].product != published || len(callbacks.operations) != callbacksBeforeClose || operationCount(callbacks.operations, ast.DatalogOpUnion) != 1 {
		t.Fatal("ground continuation retry replayed or replaced the committed rule", callbacks.operations)
	}
	must(receipt.Cleanup())
	if len(callbacks.operations) != callbacksBeforeClose || operationCount(callbacks.operations, ast.DatalogOpUnion) != 1 {
		t.Fatal("completed ground cleanup replayed callbacks", callbacks.operations)
	}

	empty, err := published.Empty()
	must(err)
	if empty {
		t.Fatal("published ground union target became empty")
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
		t.Fatal("ground temporary close fixture leaked bytes", used)
	}
}
