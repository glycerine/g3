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

func TestExternalIntervalDatalogDuplicateColumnEmptyBoundaryNativeCleanup(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var continuation *externalIntervalDatalogCartesianContinuation
	var cleanupFailure error
	active, fired := false, false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(failure memory.Failure) error {
		if active && !fired {
			fired = true
			cleanupFailure = continuation.resume()
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
	seven, eleven := value("7"), value("11")
	number, err := numbers.NewValue()
	must(err)
	must(numbers.SetString(&number, "18446744073709551617/18446744073709551619"))
	relations, err := rel.NewRelationManager(manager)
	must(err)
	state := &externalIntervalCallbackState{manager: manager, nonempty: make(map[ast.ASTHandle]bool)}
	callbacks := state.callbacks()
	reduce := callbacks.Reduce
	armed, observedOrder := false, false
	var input *rel.ExternalIntervalProduct
	callbacks.Reduce = func(declaration ast.ASTHandle, arguments []ast.ASTHandle) (ast.ASTHandle, error) {
		op, cause := manager.GetDeclKind(declaration)
		if cause != nil || !armed || op != ast.DatalogOpIsEmpty {
			return reduce(declaration, arguments)
		}
		armed = false
		state.operations = append(state.operations, op)
		_, inputErr := input.Sorts()
		_, auxiliaryErr := continuation.auxiliary.Sorts()
		observedOrder = errors.Is(inputErr, rel.ErrTableRelation) && auxiliaryErr == nil && continuation.temporary != input && continuation.retiring == nil
		// Trigger a real native guard from the helper's actual JOIN Empty
		// callback. Cleanup must preserve both JOIN and unary projection.
		if cause = runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1); cause != nil {
			return ast.ASTHandle{}, cause
		}
		var invocation ast.ASTArithmeticNumeralInvocation
		active = true
		_, cause = manager.ArithmeticNumeralWithInvocation(&number, false, &invocation)
		active = false
		cause = errors.Join(cause, runtime.SetMaxCount(0), manager.CleanupArithmeticNumeralInvocation(&invocation))
		return ast.ASTHandle{}, cause
	}
	plugin, err := rel.NewExternalRelationPlugin(relations, callbacks)
	must(err)
	source, selected, err := rel.NewSelectedExternalIntervalProduct(plugin, []ast.ASTHandle{integer, integer}, []string{"external_relation", "interval_relation"})
	must(err)
	if !selected {
		t.Fatal("duplicate fixture product not selected")
	}
	must(source.AddFact([]ast.ASTHandle{seven, eleven}))
	input, err = source.Clone()
	must(err)
	continuation = &externalIntervalDatalogCartesianContinuation{temporary: input, runtime: runtime}
	state.operations = nil
	armed = true
	empty, cause := continuation.duplicateColumn(0)
	if cause == nil || empty || !fired || !observedOrder || !errors.Is(cleanupFailure, ast.ErrASTPending) || continuation.temporary == nil || continuation.auxiliary == nil {
		t.Fatal("duplicate Empty boundary lost native cleanup owners", cause, empty, fired, observedOrder, cleanupFailure)
	}
	assertExternalIntervalRegisterOperations(t, state.operations, []int32{ast.DatalogOpProject, ast.DatalogOpJoin, ast.DatalogOpIsEmpty})
	joined, auxiliary := continuation.temporary, continuation.auxiliary
	before := len(state.operations)
	receipt := newExternalIntervalDatalogCartesianFailure(cleanupFailure, continuation)
	shown, exposed, original := receipt.Error(), receipt.Unwrap(), receipt.OriginalCause()
	must(receipt.Cleanup())
	must(receipt.Cleanup())
	if continuation.temporary != nil || continuation.auxiliary != nil || receipt.CleanupCause() != nil || receipt.Error() != shown || receipt.Unwrap() != exposed || receipt.OriginalCause() != original || len(state.operations) != before {
		t.Fatal("duplicate cleanup replayed callbacks or changed its receipt")
	}
	for _, product := range []*rel.ExternalIntervalProduct{joined, auxiliary} {
		if _, err := product.Sorts(); !errors.Is(err, rel.ErrTableRelation) {
			t.Fatal("duplicate cleanup left a product open", err)
		}
	}
	// Exercise both normal helper branches: multi-column projection and the
	// source unary alias path, which must add no projection callback.
	for _, unary := range []bool{false, true} {
		if unary {
			input, err = source.Project([]uint32{1})
		} else {
			input, err = source.Clone()
		}
		must(err)
		continuation = &externalIntervalDatalogCartesianContinuation{temporary: input, runtime: runtime}
		state.operations = nil
		empty, cause = continuation.duplicateColumn(0)
		must(cause)
		if empty || continuation.auxiliary != nil || continuation.retiring != nil {
			t.Fatal("successful duplicate left temporary ownership")
		}
		want := []int32{ast.DatalogOpJoin, ast.DatalogOpIsEmpty}
		column := 1
		if !unary {
			want = append([]int32{ast.DatalogOpProject}, want...)
			column = 2
		}
		assertExternalIntervalRegisterOperations(t, state.operations, want)
		assertExternalIntervalGroundProgramInterval(t, continuation.temporary, column, "7", "7")
		must(continuation.resume())
	}
	assertExternalIntervalGroundProgramInterval(t, source, 0, "7", "7")
	assertExternalIntervalGroundProgramInterval(t, source, 1, "11", "11")
	must(source.Close())
	must(plugin.Close())
	must(relations.Close())
	must(numbers.Del(&number))
	for i := len(roots) - 1; i >= 0; i-- {
		must(manager.DelRef(&roots[i]))
	}
	must(manager.Close())
	must(numbers.Close())
	must(symbols.Close())
	must(runtime.Close())
	if budget.Used() != 0 {
		t.Fatal("duplicate fixture leaked bytes", budget.Used())
	}
}
