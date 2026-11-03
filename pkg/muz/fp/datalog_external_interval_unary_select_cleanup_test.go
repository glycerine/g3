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

func TestExternalIntervalUnarySelectNativeCloneClosePreservesProjectedOwner(t *testing.T) {
	testExternalIntervalUnaryProjectionNativeCleanup(t, false)
}

func TestExternalIntervalUnaryFilterProjectNativeCloneClosePreservesProjectedOwner(t *testing.T) {
	testExternalIntervalUnaryProjectionNativeCleanup(t, true)
}

func testExternalIntervalUnaryProjectionNativeCleanup(t *testing.T, interpreted bool) {
	t.Helper()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var attempt *externalIntervalUnaryProjectionAttempt
	var published *rel.ExternalIntervalProduct
	var completionFailure error
	active, fired := false, false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(failure memory.Failure) error {
		if active && !fired {
			fired = true
			published, completionFailure = attempt.complete()
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
	var roots []ast.ASTRef
	value := func(text string) ast.ASTHandle {
		result, cause := manager.ArithmeticIntegerNumeralText(text)
		must(cause)
		root, cause := manager.NewRef(result)
		must(cause)
		roots = append(roots, root)
		return result
	}
	seven, eleven, five := value("7"), value("11"), value("5")
	condition := func(bound ast.ASTHandle) ast.ASTHandle {
		variable, cause := manager.MkVar(0, integer)
		must(cause)
		variableRoot, cause := manager.NewRef(variable)
		must(cause)
		guard, cause := manager.MkEq(variable, bound)
		must(cause)
		root, cause := manager.NewRef(guard)
		must(cause)
		roots = append(roots, root)
		must(manager.DelRef(&variableRoot))
		return guard
	}
	relations, err := rel.NewRelationManager(manager)
	must(err)
	callbacks := &externalIntervalCallbackState{manager: manager, nonempty: make(map[ast.ASTHandle]bool)}
	plugin, err := rel.NewExternalRelationPlugin(relations, callbacks.callbacks())
	must(err)
	source, selected, err := rel.NewSelectedExternalIntervalProduct(plugin, []ast.ASTHandle{integer, integer}, []string{"external_relation", "interval_relation"})
	must(err)
	if !selected {
		t.Fatal("select fixture product was not selected")
	}
	must(source.AddFact([]ast.ASTHandle{seven, eleven}))
	callbacks.operations = nil

	// Stage each helper's real CLONE/FILTER/PROJECT products. Native Transport
	// invokes their shared finalizer, stopping specifically at clone retirement
	// before the result can reach its caller.
	attempt = &externalIntervalUnaryProjectionAttempt{runtime: runtime}
	attempt.working, err = source.Clone()
	must(err)
	if interpreted {
		must(attempt.working.FilterInterpreted(condition(seven)))
	} else {
		must(attempt.working.FilterEqual(0, seven))
	}
	attempt.projected, err = attempt.working.Project([]uint32{0})
	must(err)
	working, projected := attempt.working, attempt.projected
	assertExternalIntervalRegisterOperations(t, callbacks.operations, []int32{
		ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpProject,
	})
	callbacksBeforeCleanup := len(callbacks.operations)
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
	if body == nil || !fired || published != nil || !errors.Is(completionFailure, ast.ErrASTPending) || attempt.working != working || attempt.projected != projected {
		t.Fatal("native clone cleanup refusal lost select ownership", body, fired, published, completionFailure)
	}
	var receipt memory.ConstructionFailure
	if !errors.As(completionFailure, &receipt) || receipt.RuntimeOwner() != runtime || !errors.Is(receipt.CleanupCause(), ast.ErrASTPending) {
		t.Fatal("select cleanup refusal lost its typed receipt", completionFailure)
	}
	shown, exposed, original := receipt.Error(), errors.Unwrap(receipt), receipt.OriginalCause()
	must(receipt.Cleanup())
	if attempt.working != nil || attempt.projected != nil || receipt.CleanupCause() != nil || receipt.Error() != shown || errors.Unwrap(receipt) != exposed || receipt.OriginalCause() != original {
		t.Fatal("select retry did not drain products with a stable receipt")
	}
	must(receipt.Cleanup())
	if len(callbacks.operations) != callbacksBeforeCleanup {
		t.Fatal("select cleanup replayed relation callbacks", callbacks.operations)
	}
	for _, product := range []*rel.ExternalIntervalProduct{working, projected} {
		if _, cause := product.Empty(); !errors.Is(cause, rel.ErrTableRelation) {
			t.Fatal("select cleanup left a private product open", cause)
		}
	}
	assertExternalIntervalGroundProgramInterval(t, source, 0, "7", "7")
	assertExternalIntervalGroundProgramInterval(t, source, 1, "11", "11")

	for _, test := range []struct {
		value ast.ASTHandle
		empty bool
	}{{value: seven}, {value: five, empty: true}} {
		callbacks.operations = nil
		var result *rel.ExternalIntervalProduct
		var cause error
		if interpreted {
			result, cause = filterProjectExternalIntervalUnary(source, condition(test.value), []uint32{0}, runtime)
		} else {
			result, cause = selectExternalIntervalUnary(source, 0, test.value, runtime)
		}
		must(cause)
		if result == nil {
			t.Fatal("select helper failed to publish a completed projection")
		}
		// Even the contradictory selection must project before its caller
		// checks emptiness; the helper itself emits no ISEMPTY callback.
		assertExternalIntervalRegisterOperations(t, callbacks.operations, []int32{
			ast.DatalogOpClone, ast.DatalogOpFilter, ast.DatalogOpProject,
		})
		empty, cause := result.Empty()
		must(cause)
		if empty != test.empty {
			t.Fatal("select projected result emptiness", empty, test.empty)
		}
		if !empty {
			assertExternalIntervalGroundProgramInterval(t, result, 0, "11", "11")
		}
		must(result.Close())
	}
	assertExternalIntervalGroundProgramInterval(t, source, 0, "7", "7")
	assertExternalIntervalGroundProgramInterval(t, source, 1, "11", "11")
	must(numbers.Del(&number))
	must(source.Close())
	must(plugin.Close())
	must(relations.Close())
	for i := len(roots) - 1; i >= 0; i-- {
		must(manager.DelRef(&roots[i]))
	}
	must(manager.Close())
	must(numbers.Close())
	must(symbols.Close())
	must(runtime.Close())
	if used := budget.Used(); used != 0 {
		t.Fatal("select cleanup fixture leaked bytes", used)
	}
}
