package transforms

// Source: src/muz/rel/dl_mk_simple_joins.cpp planner::m_pinned ownership.
// The planner is local to mk_simple_joins::operator().

import (
	"testing"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// TestSimpleJoinsReleasesPlannerPinsOnReturn checks that the planner-local
// m_pinned terms, including apply_binary_rule's new literals, are released
// when Transform returns, while the produced rules keep their own terms.
func TestSimpleJoinsReleasesPlannerPinsOnReturn(t *testing.T) {
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	budget := arena.NewBudget(1 << 28)
	runtime, err := memory.NewRuntime(memory.RuntimeConfig{Budget: budget, SizeTBits: 64})
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
	plugin := NewSimpleJoinsPlugin(m, rules, 0)
	defer func() { must(plugin.Close()) }()
	transformed, err := plugin.Transform(set)
	must(err)
	if transformed == nil || len(plugin.FreshPredicates()) != 1 {
		t.Fatal("fixture did not create a join")
	}
	defer func() { must(transformed.Close()) }()
	if len(plugin.keyPins) != 0 {
		t.Fatal("planner pins outlived the transformation", len(plugin.keyPins))
	}
	fresh := plugin.FreshPredicates()[0]
	origin, err := m.GetDecl(apps[3])
	must(err)
	if actual, found := plugin.FreshPredicateOrigin(fresh); !found || actual != origin {
		t.Fatal("join origin missing", actual, found)
	}
	joined, err := transformed.Rule(0)
	must(err)
	if n, e := joined.PositiveTailSize(); e != nil || n != 2 {
		t.Fatal("join did not reduce positive tails", n, e)
	}
	must(joined.Close())
	must(plugin.Close())
	must(plugin.Close())
	if !plugin.closed || plugin.fresh != nil || plugin.origins != nil {
		t.Fatal("close did not release plugin state")
	}
}
