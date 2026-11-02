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

func TestExternalRelationNewTotalSourceConstructionAndFailure(t *testing.T) {
	f := newTableRelationFixture(t)
	refusal := errors.New("total construction callback refusal")
	failing := int32(-1)
	var operations []int32
	plugin, err := NewExternalRelationPlugin(f.relations, ExternalRelationCallbacks{
		ReduceAssign: func(declaration ast.ASTHandle, _, _ []ast.ASTHandle) error {
			kind, cause := f.ast.GetDeclKind(declaration)
			if cause != nil {
				return cause
			}
			operations = append(operations, kind)
			if kind == failing {
				return refusal
			}
			return nil
		},
		Reduce: func(declaration ast.ASTHandle, arguments []ast.ASTHandle) (ast.ASTHandle, error) {
			kind, cause := f.ast.GetDeclKind(declaration)
			if cause != nil {
				return ast.ASTHandle{}, cause
			}
			operations = append(operations, kind)
			if kind == failing {
				return ast.ASTHandle{}, refusal
			}
			return arguments[0], nil
		},
	})
	astTestMust(t, err)

	total, err := plugin.NewTotal([]ast.ASTHandle{f.boolean})
	astTestMust(t, err)
	if total == nil || len(operations) != 2 || operations[0] != ast.DatalogOpEmpty || operations[1] != ast.DatalogOpComplement || plugin.users != 1 {
		t.Fatal("NewTotal did not perform Empty(signature).Complement", operations, plugin.users)
	}
	astTestMust(t, total.Close())

	operations = nil
	failing = ast.DatalogOpEmpty
	total, err = plugin.NewTotal([]ast.ASTHandle{f.boolean})
	if total != nil || !errors.Is(err, refusal) || plugin.users != 0 || len(operations) != 1 {
		t.Fatal("empty refusal published a total prefix", total, err, plugin.users, operations)
	}

	operations = nil
	failing = ast.DatalogOpComplement
	total, err = plugin.NewTotal([]ast.ASTHandle{f.boolean})
	if total != nil || !errors.Is(err, refusal) || plugin.users != 0 || len(operations) != 2 {
		t.Fatal("complement refusal published or leaked a total prefix", total, err, plugin.users, operations)
	}
	astTestMust(t, plugin.Close())
}

func TestExternalRelationNewTotalCleanupRefusalIsRetryable(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var owner *externalRelationTotalOwner
	var failure error
	active := false
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(value memory.Failure) error {
		if active && failure == nil {
			failure = finishExternalRelationTotal(owner, nil)
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
	empty, err := plugin.NewEmpty(nil)
	must(err)
	total, err := empty.Complement()
	must(err)
	owner = &externalRelationTotalOwner{plugin: plugin, empty: empty, total: total}

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
	if body == nil || failure == nil || owner.total == nil || owner.empty == nil || plugin.users != 2 {
		t.Fatal("native total cleanup did not preserve both concrete owners", body, failure, plugin.users)
	}
	var receipt memory.ConstructionFailure
	if !errors.As(failure, &receipt) {
		t.Fatal("total cleanup lost typed retry owner", failure)
	}
	message, exposed := receipt.Error(), receipt.Unwrap()
	must(receipt.Cleanup())
	if receipt.Error() != message || receipt.Unwrap() != exposed || receipt.CleanupCause() != nil || owner.total != nil || owner.empty != nil || plugin.users != 0 {
		t.Fatal("total retry changed identity or left concrete owners")
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
		t.Fatal("total refusal fixture leaked bytes", used)
	}
}
