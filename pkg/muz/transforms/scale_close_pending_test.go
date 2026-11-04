package transforms

// Scale temporary owners must remain reachable when cleanup is refused.

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func TestScaleCloseRetainsRefusedOwners(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var plugin *ScalePlugin
	active, callbacks, intact := false, 0, true
	var original ast.ASTRef
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active {
			callbacks++
			for i := 0; i < 2; i++ {
				cause := plugin.Close()
				intact = intact && errors.Is(cause, ast.ErrASTPending) && !plugin.closed && len(plugin.pendingPins) == 1 && plugin.pendingPins[0] == original && len(plugin.pendingRules) == 1 && len(plugin.pendingSets) == 1
			}
		}
		return f
	}})
	must(err)
	numbers, err := numeral.NewRationalService(numeral.RationalServiceConfig{Integers: numeral.IntegerConfig{Memory: runtime, Profile: memory.SmallAllocatorProfile{PointerBits: 64, Debug: true}, Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
	must(err)
	symbols, err := symbol.NewManager(symbol.Config{Budget: budget})
	must(err)
	m, err := ast.NewASTManager(ast.ASTManagerConfig{Symbols: symbols, Numerals: numbers, Budget: budget})
	must(err)
	defer func() {
		must(m.Close())
		must(numbers.Close())
		must(symbols.Close())
		must(runtime.Close())
		if budget.Used() != 0 {
			t.Fatal("fixture leaked bytes", budget.Used())
		}
	}()
	_, err = m.RegisterArithmeticPlugin()
	must(err)
	boolean, err := m.MkBoolSort()
	must(err)
	rules, err := base.NewRuleManager(m)
	must(err)
	defer func() { must(rules.Close()) }()
	set, err := base.NewRuleSet(rules)
	must(err)
	name, err := symbols.Intern("pending-scale")
	must(err)
	decl, err := m.MkFuncDecl(name, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(rules.RegisterPredicate(decl))
	head, err := m.MkApp(decl, nil)
	must(err)
	setRule, err := rules.NewRule(head, nil, nil, "set-pending")
	must(err)
	must(set.Add(setRule))
	must(setRule.Close())
	rule, err := rules.NewRule(head, nil, nil, "pending")
	must(err)
	plugin = NewScalePlugin(m, rules, 0)
	original, err = m.NewRef(head)
	must(err)
	plugin.pendingPins = append(plugin.pendingPins, original)
	plugin.pendingRules = append(plugin.pendingRules, rule)
	plugin.pendingSets = append(plugin.pendingSets, set)
	value, err := numbers.NewValue()
	must(err)
	defer func() { must(numbers.Del(&value)) }()
	must(numbers.SetString(&value, "18446744073709551617/18446744073709551619"))
	must(runtime.SetMaxCount(uint64(runtime.State().GlobalCount) + 1))
	var invocation ast.ASTArithmeticNumeralInvocation
	active = true
	_, body := m.ArithmeticNumeralWithInvocation(&value, false, &invocation)
	active = false
	must(runtime.SetMaxCount(0))
	must(m.CleanupArithmeticNumeralInvocation(&invocation))
	if body == nil || callbacks == 0 || !intact {
		t.Fatal("refused cleanup lost scale owners", body, callbacks, intact)
	}
	if next, cause := plugin.Transform(set); !errors.Is(cause, base.ErrRuleTransformerOwner) || next != nil {
		t.Fatal("closing plugin accepted transformation", cause)
	}
	must(plugin.Close())
	must(plugin.Close())
	if !plugin.closed || len(plugin.pendingPins) != 0 || len(plugin.pendingRules) != 0 || len(plugin.pendingSets) != 0 {
		t.Fatal("retry did not release scale owners")
	}
}
