package rel

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestExternalInterpretedFilterCleanupRefusalKeepsRetryOwner(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var refuse func() error
	var failure error
	active, refusing := false, false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(value memory.Failure) error {
		if active && !refusing && failure == nil {
			refusing = true
			failure = refuse()
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
	relations, err := NewRelationManager(manager)
	must(err)
	number, err := numbers.NewValue()
	must(err)
	must(numbers.SetString(&number, "18446744073709551617/18446744073709551619"))
	filterCalled := false
	plugin, err := NewExternalRelationPlugin(relations, ExternalRelationCallbacks{ReduceAssign: func(declaration ast.ASTHandle, _, _ []ast.ASTHandle) error {
		kind, cause := manager.GetDeclKind(declaration)
		if cause == nil && kind == ast.DatalogOpFilter {
			filterCalled = true
		}
		return cause
	}})
	must(err)
	boolean, err := manager.MkBoolSort()
	must(err)
	truth, err := manager.MkTrue()
	must(err)
	variable, err := manager.MkVar(0, boolean)
	must(err)
	condition, err := manager.MkEq(variable, truth)
	must(err)
	conditionRef, err := manager.NewRef(condition)
	must(err)
	relation, err := plugin.NewEmpty([]ast.ASTHandle{boolean})
	must(err)
	binding, err := manager.MkVar(0, boolean)
	must(err)
	bindingRef, err := manager.NewRef(binding)
	must(err)
	substitution, err := astrewriter.NewVarSubstitution(manager, false)
	must(err)
	bound, err := manager.NewRef(ast.ASTHandle{})
	must(err)
	must(substitution.Substitute(condition, []ast.ASTHandle{binding}, bound))
	boundHandle, err := manager.RefGet(bound)
	must(err)
	must(relation.FilterInterpreted(boundHandle))
	numericHandle, err := manager.ArithmeticNumeral(&number, false)
	must(err)
	numericRef, err := manager.NewRef(numericHandle)
	must(err)
	attempt := &externalInterpretedAttempt{manager: manager, substitution: substitution, roots: []ast.ASTRef{bindingRef, bound, numericRef}}
	refuse = func() error { return finishExternalInterpretedAttempt(attempt, nil) }
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	active = true
	var invocation ast.ASTArithmeticNumeralInvocation
	_, body := manager.ArithmeticNumeralWithInvocation(&number, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	must(manager.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || !filterCalled || !errors.Is(failure, ast.ErrASTPending) {
		t.Fatal("native refusal did not reach interpreted-filter finalizer", body, filterCalled, failure)
	}
	var receipt memory.ConstructionFailure
	if !errors.As(failure, &receipt) {
		t.Fatal("interpreted-filter refusal lost retry owner", failure)
	}
	owned, ok := receipt.(*externalInterpretedCleanupFailure)
	if !ok || owned.attempt == nil || owned.RuntimeOwner() != runtime || owned.OriginalCause() == nil || owned.CleanupCause() == nil || owned.attempt.substitution == nil && len(owned.attempt.roots) == 0 {
		t.Fatal("interpreted-filter receipt lost concrete owners", owned)
	}
	message, exposed := receipt.Error(), receipt.Unwrap()
	must(receipt.Cleanup())
	if receipt.Error() != message || receipt.Unwrap() != exposed || !errors.Is(receipt, ast.ErrASTPending) || receipt.CleanupCause() != nil {
		t.Fatal("interpreted-filter receipt changed stable error identity")
	}
	if owned.attempt.substitution != nil || len(owned.attempt.roots) != 0 {
		t.Fatal("interpreted-filter cleanup retry left owners")
	}
	must(receipt.Cleanup())
	must(relation.Close())
	must(plugin.Close())
	must(manager.DelRef(&conditionRef))
	must(relations.Close())
	must(numbers.Del(&number))
	must(manager.Close())
	must(numbers.Close())
	must(symbols.Close())
	must(runtime.Close())
	if used := budget.Used(); used != 0 {
		t.Fatal("interpreted-filter refusal fixture leaked bytes", used)
	}
}
