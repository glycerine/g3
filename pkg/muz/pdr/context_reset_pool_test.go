package pdr

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func TestContextResetReleasesSolverPoolsAndPreservesStatistics(t *testing.T) {
	m, owner, old, _, closeBase := closureFixture(t)
	defer func() { closureMust(t, old.Close()); closureMust(t, owner.Close()); closeBase() }()
	must := func(err error) { t.Helper(); closureMust(t, err) }
	context, err := NewContext(m, nil, 2, ReachableConstraintCache, owner.contexts.config)
	must(err)
	defer func() { must(context.Close()) }()
	declaration, err := old.Head()
	must(err)
	zero, err := m.ArithmeticRealNumeralText("0")
	must(err)
	head, err := m.MkApp(declaration, []ast.ASTHandle{zero})
	must(err)
	rules, err := base.NewRuleManager(m)
	must(err)
	defer func() { must(rules.Close()) }()
	must(rules.RegisterPredicate(declaration))
	set, err := base.NewRuleSet(rules)
	must(err)
	defer func() { must(set.Close()) }()
	rule, err := rules.NewRule(head, nil, nil, "reset-pool-fact")
	must(err)
	must(set.Add(rule))
	must(rule.Close())
	checks := func() uint32 {
		t.Helper()
		stats := statistics.New()
		must(context.CollectStatistics(stats))
		var count uint32
		for index := 0; index < stats.Size(); index++ {
			if stats.IsUint(index) && stats.Key(index) == "num checks" {
				count += stats.UintValue(index)
			}
		}
		return count
	}
	var previous uint32
	for round := 0; round < 5; round++ {
		must(context.Initialize(set))
		status, err := context.Solve(declaration)
		must(err)
		if status != lbool.True {
			t.Fatal("reset-pool fact is unreachable", round, status)
		}
		pool := context.owner.contexts
		if len(pool.solvers) == 0 || len(pool.solvers) > 2 {
			t.Fatal("solver pool grew across resets", round, len(pool.solvers))
		}
		solver := pool.solvers[0]
		before := checks()
		if before <= previous {
			t.Fatal("new solve did not advance cumulative checks", round, before, previous)
		}
		must(context.Reset())
		if pool.active != 0 || len(pool.solvers) != 0 || len(pool.predicates) != 0 {
			t.Fatal("reset left solver-pool resources alive", round, pool.active, len(pool.solvers), len(pool.predicates))
		}
		if _, err := solver.ArithmeticState(); err == nil {
			t.Fatal("reset left a retired solver open")
		}
		if got := checks(); got != before {
			t.Fatal("reset lost or duplicated solver statistics", round, got, before)
		}
		must(context.ResetStatistics())
		if got := checks(); got != before {
			t.Fatal("statistics reset changed source cumulative kernel counters", got, before)
		}
		previous = before
	}
}
