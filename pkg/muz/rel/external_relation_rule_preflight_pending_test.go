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

func TestExternalRulePreflightCleanupPreservesNestedTotalsAndSingletons(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var refused error
	var during func() error
	active := false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(value memory.Failure) error {
		if active && refused == nil {
			refused = during()
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
	plugin, err := NewExternalRelationPlugin(relations, ExternalRelationCallbacks{Reduce: func(_ ast.ASTHandle, arguments []ast.ASTHandle) (ast.ASTHandle, error) {
		return arguments[0], nil
	}})
	must(err)
	boolean, err := manager.MkBoolSort()
	must(err)
	truth, err := manager.MkTrue()
	must(err)
	newTotal := func(sorts []ast.ASTHandle) *ExternalRelation {
		t.Helper()
		total, cause := plugin.NewTotal(sorts)
		must(cause)
		return total
	}
	newSingleton := func() *ExternalRelation {
		t.Helper()
		singleton, cause := plugin.NewEmpty([]ast.ASTHandle{boolean})
		must(cause)
		must(singleton.AddFact([]ast.ASTHandle{truth}))
		return singleton
	}
	number, err := numbers.NewValue()
	must(err)
	must(numbers.SetString(&number, "18446744073709551617/18446744073709551619"))
	trigger := func() memory.ConstructionFailure {
		t.Helper()
		refused = nil
		must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
		active = true
		var invocation ast.ASTArithmeticNumeralInvocation
		_, body := manager.ArithmeticNumeralWithInvocation(&number, false, &invocation)
		active = false
		must(runtime.SetMaxCount(0))
		must(manager.CleanupArithmeticNumeralInvocation(&invocation))
		if body == nil || refused == nil {
			t.Fatal("native preflight cleanup was not refused", body, refused)
		}
		var receipt memory.ConstructionFailure
		if !errors.As(refused, &receipt) || receipt.RuntimeOwner() != runtime {
			t.Fatal("preflight refusal lost typed runtime owner", refused, receipt)
		}
		return receipt
	}
	checkRetry := func(receipt memory.ConstructionFailure, owner *externalRulePreflightOwner) {
		t.Helper()
		message, exposed := receipt.Error(), receipt.Unwrap()
		must(receipt.Cleanup())
		if receipt.Error() != message || receipt.Unwrap() != exposed || receipt.CleanupCause() != nil || owner.nested != nil || plugin.users != 0 {
			t.Fatal("preflight retry changed identity or left owners")
		}
		for _, relation := range owner.totals {
			if relation != nil {
				t.Fatal("preflight retry left total")
			}
		}
		for _, relation := range owner.singletons {
			if relation != nil {
				t.Fatal("preflight retry left singleton")
			}
		}
		must(receipt.Cleanup())
	}

	// Totals were constructed after singletons and therefore retire first.
	firstSingleton, secondSingleton := newSingleton(), newSingleton()
	firstTotal, secondTotal := newTotal([]ast.ASTHandle{boolean}), newTotal(nil)
	owner := &externalRulePreflightOwner{plugin: plugin, totals: []*ExternalRelation{firstTotal, secondTotal}, singletons: []*ExternalRelation{firstSingleton, secondSingleton}}
	during = func() error { return finishExternalRulePreflight(owner, nil) }
	receipt := trigger()
	if owner.totals[0] != firstTotal || owner.totals[1] != secondTotal || owner.singletons[0] != firstSingleton || owner.singletons[1] != secondSingleton || plugin.users != 4 {
		t.Fatal("total frontier refusal lost preflight owners", plugin.users)
	}
	checkRetry(receipt, owner)

	// Once totals are absent, the singleton reverse frontier is independently
	// retryable rather than being lost by a raw deferred Close.
	firstSingleton, secondSingleton = newSingleton(), newSingleton()
	owner = &externalRulePreflightOwner{plugin: plugin, singletons: []*ExternalRelation{firstSingleton, secondSingleton}}
	during = func() error { return finishExternalRulePreflight(owner, nil) }
	receipt = trigger()
	if owner.singletons[0] != firstSingleton || owner.singletons[1] != secondSingleton || plugin.users != 2 {
		t.Fatal("singleton frontier refusal lost concrete owners", plugin.users)
	}
	checkRetry(receipt, owner)

	// A constructor receipt is always drained before later totals and earlier
	// singleton resources.
	singleton := newSingleton()
	total := newTotal([]ast.ASTHandle{boolean})
	childEmpty, err := plugin.NewEmpty(nil)
	must(err)
	childTotal, err := childEmpty.Complement()
	must(err)
	childCause := errors.New("nested preflight constructor failed")
	nested := &externalRelationTotalFailure{
		owner:   &externalRelationTotalOwner{plugin: plugin, empty: childEmpty, total: childTotal},
		cause:   childCause,
		shown:   childCause,
		message: childCause.Error(),
	}
	owner = &externalRulePreflightOwner{plugin: plugin, totals: []*ExternalRelation{total}, singletons: []*ExternalRelation{singleton}}
	during = func() error { return finishExternalRulePreflight(owner, nested) }
	receipt = trigger()
	if owner.nested != nested || owner.totals[0] != total || owner.singletons[0] != singleton || plugin.users != 4 {
		t.Fatal("nested refusal touched later preflight resources", plugin.users)
	}
	checkRetry(receipt, owner)

	must(plugin.Close())
	must(relations.Close())
	must(numbers.Del(&number))
	must(manager.Close())
	must(numbers.Close())
	must(symbols.Close())
	must(runtime.Close())
	if used := budget.Used(); used != 0 {
		t.Fatal("preflight refusal fixture leaked bytes", used)
	}
}
