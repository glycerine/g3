package transforms

// Source: src/muz/transforms/dl_mk_filter_rules.cpp reset and filter cache ownership.
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

func testFilterRulesCacheRefusal(t *testing.T, duringTransform bool) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	var plugin *FilterRulesPlugin
	active, callbacks, intact := false, 0, true
	var original []filterRuleCacheEntry
	var set *base.RuleSet
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64, Transport: func(f memory.Failure) error {
		if active {
			callbacks++
			for i := 0; i < 2; i++ {
				var cause error
				if duringTransform {
					_, cause = plugin.Transform(set)
				} else {
					cause = plugin.Close()
				}
				intact = intact && errors.Is(cause, ast.ErrASTPending) && !plugin.closed && len(plugin.cache) == len(original)
				for j := range plugin.cache {
					intact = intact && plugin.cache[j].pin == original[j].pin && plugin.cache[j].origin == original[j].origin
					origin, visible := plugin.FreshPredicateOrigin(original[j].declaration)
					intact = intact && visible == duringTransform
					if visible {
						intact = intact && origin == original[j].origin
					}
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
	set, err = base.NewRuleSet(rules)
	must(err)
	defer func() { must(set.Close()) }()
	zero, err := m.ArithmeticIntegerNumeralText("0")
	must(err)
	integer, err := m.GetSort(zero)
	must(err)
	n, err := symbols.Intern("pending-filter-P")
	must(err)
	d, err := m.MkFuncDecl(n, []ast.ASTHandle{integer}, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(rules.RegisterPredicate(d))
	tail, err := m.MkApp(d, []ast.ASTHandle{zero})
	must(err)
	n, err = symbols.Intern("pending-filter-Goal")
	must(err)
	d, err = m.MkFuncDecl(n, nil, boolean, ast.ASTFuncDeclInfo{})
	must(err)
	must(rules.RegisterPredicate(d))
	head, err := m.MkApp(d, nil)
	must(err)
	rule, err := rules.NewRule(head, []ast.ASTHandle{tail}, nil, "value-tail")
	must(err)
	must(set.Add(rule))
	must(rule.Close())
	plugin = NewFilterRulesPlugin(m, rules, 0)
	defer func() { must(plugin.Close()) }()
	transformed, err := plugin.Transform(set)
	must(err)
	if transformed == nil || len(plugin.cache) != 1 || len(plugin.FreshPredicates()) != 1 {
		t.Fatal("fixture did not create cached filter")
	}
	defer func() { must(transformed.Close()) }()
	original = append([]filterRuleCacheEntry(nil), plugin.cache...)
	tailDeclaration, err := m.GetDecl(tail)
	must(err)
	if origin, found := plugin.FreshPredicateOrigin(original[0].declaration); !found || origin != tailDeclaration {
		t.Fatal("filter origin missing before cleanup", origin, found)
	}
	if count, e := transformed.Len(); e != nil || count != 2 {
		t.Fatal("filter did not create definition and rewritten rule", count, e)
	}
	if arity, e := m.GetArity(original[0].declaration); e != nil || arity != 0 {
		t.Fatal("value filter should be nullary", arity, e)
	}
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
		t.Fatal("refused cleanup lost filter cache", body, callbacks, intact)
	}
	for _, entry := range original {
		h, e := m.RefGet(entry.pin)
		must(e)
		_, e = m.GetKind(h)
		must(e)
	}
	if duringTransform {
		next, e := plugin.Transform(set)
		must(e)
		if next == nil || len(plugin.FreshPredicates()) != 1 {
			t.Fatal("cache reset retry did not rebuild filter")
		}
		must(next.Close())
	} else {
		if len(plugin.FreshPredicates()) != 0 {
			t.Fatal("closing plugin published fresh predicates")
		}
		if next, cause := plugin.Transform(set); !errors.Is(cause, base.ErrRuleTransformerOwner) || next != nil {
			t.Fatal("closing plugin accepted transformation", cause)
		}
	}
	must(plugin.Close())
	must(plugin.Close())
	if !plugin.closed || plugin.cache != nil {
		t.Fatal("retry did not release filter cache")
	}
}

func TestFilterRulesCacheCleanupRefusal(t *testing.T) {
	t.Run("close", func(t *testing.T) { testFilterRulesCacheRefusal(t, false) })
	t.Run("next-transform", func(t *testing.T) { testFilterRulesCacheRefusal(t, true) })
}
