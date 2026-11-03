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

func TestExternalRuleSeedAggregateCleanupPreservesNestedAndTotals(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var owner *externalRuleSeedOwner
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
	newTotal := func(sorts []ast.ASTHandle) *ExternalRelation {
		t.Helper()
		total, cause := plugin.NewTotal(sorts)
		must(cause)
		return total
	}
	number, err := numbers.NewValue()
	must(err)
	must(numbers.SetString(&number, "18446744073709551617/18446744073709551619"))
	trigger := func() (error, memory.ConstructionFailure) {
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
			t.Fatal("native aggregate cleanup was not refused", body, refused)
		}
		var receipt memory.ConstructionFailure
		if !errors.As(refused, &receipt) || receipt.RuntimeOwner() != runtime {
			t.Fatal("aggregate refusal lost typed runtime owner", refused, receipt)
		}
		return body, receipt
	}
	boolean, err := manager.MkBoolSort()
	must(err)

	// Exercise the aggregate frontier itself. The last-constructed nullary
	// total refuses first; both it and the preceding unary total remain live.
	first, second := newTotal([]ast.ASTHandle{boolean}), newTotal(nil)
	owner = &externalRuleSeedOwner{plugin: plugin, totals: []*ExternalRelation{first, second}}
	during = func() error { return finishExternalRuleSeed(owner, nil) }
	body, receipt := trigger()
	if owner.nested != nil || owner.totals[0] != first || owner.totals[1] != second || plugin.users != 2 {
		t.Fatal("direct aggregate refusal lost unary or nullary owner", body, owner.totals, plugin.users)
	}
	message, exposed := receipt.Error(), receipt.Unwrap()
	must(receipt.Cleanup())
	if receipt.Error() != message || receipt.Unwrap() != exposed || receipt.CleanupCause() != nil || owner.totals[0] != nil || owner.totals[1] != nil || plugin.users != 0 {
		t.Fatal("direct aggregate retry changed identity or left owners")
	}
	must(receipt.Cleanup())

	// A nested constructor receipt precedes the aggregate totals and must be
	// drained before either aggregate owner is touched.
	first, second = newTotal([]ast.ASTHandle{boolean}), newTotal(nil)
	childEmpty, err := plugin.NewEmpty(nil)
	must(err)
	childTotal, err := childEmpty.Complement()
	must(err)
	childCause := errors.New("nested total construction failed")
	nested := &externalRelationTotalFailure{
		owner:   &externalRelationTotalOwner{plugin: plugin, empty: childEmpty, total: childTotal},
		cause:   childCause,
		shown:   childCause,
		message: childCause.Error(),
	}
	owner = &externalRuleSeedOwner{plugin: plugin, totals: []*ExternalRelation{first, second}}
	during = func() error { return finishExternalRuleSeed(owner, nested) }
	body, receipt = trigger()
	if owner.nested != nested || owner.totals[0] != first || owner.totals[1] != second || plugin.users != 4 {
		t.Fatal("nested aggregate refusal lost concrete owners", body, owner.nested, plugin.users)
	}
	message, exposed = receipt.Error(), receipt.Unwrap()
	must(receipt.Cleanup())
	if receipt.Error() != message || receipt.Unwrap() != exposed || receipt.CleanupCause() != nil || owner.nested != nil || owner.totals[0] != nil || owner.totals[1] != nil || plugin.users != 0 {
		t.Fatal("nested aggregate retry changed identity or left owners")
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
		t.Fatal("aggregate seed refusal fixture leaked bytes", used)
	}
}
