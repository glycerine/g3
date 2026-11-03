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

func TestExternalIntervalDatalogNaryJoinedCloseBeforePublicationRetry(t *testing.T) {
	testExternalIntervalDatalogNaryCloseRetry(t, false)
}

func TestExternalIntervalDatalogNarySingletonCloseAfterPublicationRetry(t *testing.T) {
	testExternalIntervalDatalogNaryCloseRetry(t, true)
}

func testExternalIntervalDatalogNaryCloseRetry(t *testing.T, singletonFrontier bool) {
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
	sorts := []ast.ASTHandle{integer, integer, integer}
	kinds := []string{"external_relation", "interval_relation"}
	name, err := symbols.Intern("external-interval-ground-nary-close")
	must(err)
	predicate, err := manager.MkFuncDecl(name, sorts, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	relations, err := rel.NewRelationManager(manager)
	must(err)
	callbacks := &externalIntervalCallbackState{manager: manager, nonempty: make(map[ast.ASTHandle]bool)}
	plugin, err := rel.NewExternalRelationPlugin(relations, callbacks.callbacks())
	must(err)
	activation, err := newExternalIntervalDatalogActivation(manager, plugin, []externalIntervalDatalogPredicate{{
		declaration: predicate, sorts: sorts, kinds: kinds,
	}})
	must(err)
	first, err := manager.ArithmeticIntegerNumeralText("17")
	must(err)
	second, err := manager.ArithmeticIntegerNumeralText("19")
	must(err)
	values := []ast.ASTHandle{first, second, first}
	application, err := manager.MkApp(predicate, values)
	must(err)
	must(activation.installExternalIntervalRulePlans(nil, &externalIntervalDatalogGroundRule{
		head: predicate, application: application, values: values,
	}, nil))

	old := activation.entries[predicate].product
	registers, err := activation.loadExternalIntervalDatalogRegisters([]ast.ASTHandle{predicate})
	must(err)
	tuple := &externalIntervalDatalogGroundTuple{}
	must(tuple.assemble(plugin, kinds, sorts, values))
	if tuple.joined == nil || len(tuple.singletons) != 2 {
		t.Fatal("n-ary tuple did not own its joined result and unique singleton cache")
	}
	joined := tuple.joined
	lastSingleton := tuple.singletons[len(tuple.singletons)-1]
	target, err := registers.ensureRelation(predicate)
	must(err)
	must(target.Union(tuple.current, nil, false))
	continuation = &externalIntervalDatalogGroundContinuation{
		registers: registers, tuple: tuple, runtime: runtime, publish: true,
	}
	if singletonFrontier {
		// The rule DEALLOC and termination STORE have completed. Leave an
		// empty STORE frontier so resume must finish it before cache cleanup.
		must(tuple.closeJoined())
		must(registers.store())
		continuation.registers = &externalIntervalDatalogRegisters{
			activation: activation, runtime: runtime,
			indices: make(map[ast.ASTHandle]int), published: true, retireNext: -1,
		}
	}
	storeFrontier := continuation.registers
	before := activation.entries[predicate].product
	callbacksBeforeClose := len(callbacks.operations)
	if singletonFrontier && before == old {
		t.Fatal("precondition: singleton cleanup started before publication")
	}
	if !singletonFrontier && before != old {
		t.Fatal("precondition: joined cleanup started after publication")
	}

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
	if body == nil || !fired || !errors.Is(closeFailure, ast.ErrASTPending) || continuation.tuple != tuple {
		t.Fatal("native n-ary cleanup refusal was not preserved", body, fired, closeFailure)
	}
	if singletonFrontier {
		if continuation.registers != nil || !storeFrontier.closed || tuple.joined != nil || len(tuple.singletons) != 2 || tuple.singletons[1] != lastSingleton {
			t.Fatal("singleton refusal lost its cache owner or unfinished STORE frontier")
		}
	} else if continuation.registers != registers || registers.published || registers.closed || tuple.joined != joined || len(tuple.singletons) != 2 {
		t.Fatal("joined refusal lost its owner or published before DEALLOC completed")
	}
	if activation.entries[predicate].product != before || len(callbacks.operations) != callbacksBeforeClose {
		t.Fatal("refused cleanup changed publication or replayed callbacks", callbacks.operations)
	}

	pending := newExternalIntervalDatalogGroundFailure(closeFailure, continuation)
	activation.pending = pending
	activation.pendingCompletesRule = true
	activation.pendingGround = activation.ground
	var receipt memory.ConstructionFailure
	if !errors.As(pending, &receipt) || receipt.RuntimeOwner() != runtime || !errors.Is(receipt.CleanupCause(), ast.ErrASTPending) {
		t.Fatal("n-ary refusal lost the typed cleanup receipt", pending)
	}
	shown, exposed, original := receipt.Error(), receipt.Unwrap(), receipt.OriginalCause()
	must(activation.executeExternalIntervalIdentity())
	if activation.pending != nil || activation.pendingCompletesRule || activation.pendingGround != nil || continuation.registers != nil || continuation.tuple != nil || tuple.joined != nil || len(tuple.singletons) != 0 || tuple.cache != nil {
		t.Fatal("n-ary continuation did not drain every owner")
	}
	if receipt.Error() != shown || receipt.Unwrap() != exposed || receipt.OriginalCause() != original || receipt.CleanupCause() != nil {
		t.Fatal("n-ary continuation changed receipt identity")
	}
	published := activation.entries[predicate].product
	if published == old || singletonFrontier && published != before || len(callbacks.operations) != callbacksBeforeClose || operationCount(callbacks.operations, ast.DatalogOpJoin) != 2 || operationCount(callbacks.operations, ast.DatalogOpUnion) != 1 {
		t.Fatal("n-ary retry failed publication or replayed rule operations", callbacks.operations)
	}
	must(receipt.Cleanup())
	if activation.entries[predicate].product != published || len(callbacks.operations) != callbacksBeforeClose {
		t.Fatal("idempotent cleanup changed the published result")
	}
	empty, err := published.Empty()
	must(err)
	if empty {
		t.Fatal("n-ary published tuple became empty")
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
		t.Fatal("n-ary cleanup fixture leaked bytes", used)
	}
}
