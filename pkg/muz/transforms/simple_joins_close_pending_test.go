package transforms

// Source: src/muz/rel/dl_mk_simple_joins.cpp planner::m_pinned ownership.
// Its Go converter owners must remain reachable when native cleanup is refused.

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

func TestSimpleJoinsClosePreservesAuxiliaryPins(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var plugin *SimpleJoinsPlugin
	active, callbacks, intact := false, 0, true
	var original []ast.ASTRef
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active {
			callbacks++
			for i := 0; i < 2; i++ {
				cause := plugin.Close()
				intact = intact && errors.Is(cause, ast.ErrASTPending) && !plugin.closed && len(plugin.pins) == len(original)
				for j := range plugin.pins {
					intact = intact && plugin.pins[j] == original[j]
				}
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
	defer func() { must(set.Close()) }()
	apps := make([]ast.ASTHandle, 4)
	for i, name := range []string{"A", "B", "C", "Goal"} {
		n, e := symbols.Intern("pending-join-" + name)
		must(e)
		d, e := m.MkFuncDecl(n, nil, boolean, ast.ASTFuncDeclInfo{})
		must(e)
		must(rules.RegisterPredicate(d))
		apps[i], e = m.MkApp(d, nil)
		must(e)
	}
	rule, err := rules.NewRule(apps[3], apps[:3], nil, "three-tail")
	must(err)
	must(set.Add(rule))
	must(rule.Close())
	plugin = NewSimpleJoinsPlugin(m, rules, 0)
	defer func() { must(plugin.Close()) }()
	transformed, err := plugin.Transform(set)
	must(err)
	if transformed == nil || len(plugin.pins) == 0 || len(plugin.FreshPredicates()) != 1 {
		t.Fatal("fixture did not create pinned join")
	}
	defer func() { must(transformed.Close()) }()
	original = append([]ast.ASTRef(nil), plugin.pins...)
	joined, err := transformed.Rule(0)
	must(err)
	if n, e := joined.PositiveTailSize(); e != nil || n != 2 {
		t.Fatal("join did not reduce positive tails", n, e)
	}
	must(joined.Close())
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
		t.Fatal("refused close lost join pins", body, callbacks, intact)
	}
	if len(plugin.FreshPredicates()) != 0 {
		t.Fatal("closing plugin published fresh predicates")
	}
	if next, cause := plugin.Transform(set); !errors.Is(cause, base.ErrRuleTransformerOwner) || next != nil {
		t.Fatal("closing plugin accepted transformation", cause)
	}
	for _, pin := range original {
		h, e := m.RefGet(pin)
		must(e)
		_, e = m.GetKind(h)
		must(e)
	}
	must(plugin.Close())
	must(plugin.Close())
	if !plugin.closed || plugin.pins != nil || plugin.fresh != nil {
		t.Fatal("retry did not release join pins")
	}
}
