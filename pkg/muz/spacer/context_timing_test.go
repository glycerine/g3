package spacer

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
	"github.com/glycerine/g3/pkg/util/stopwatch"
)

func TestSpacerContextSourceTimersAndSnapshotReset(t *testing.T) {
	f := newContextAnswerFixture(t, false)
	c, m := f.context, f.context.manager
	for _, watch := range []*stopwatch.Stopwatch{&c.timers.initRules, &c.timers.solve, &c.timers.propagate, &c.timers.reach, &c.timers.isReach, &c.timers.children} {
		var ticks uint64
		*watch = stopwatch.New(func() (uint64, error) { ticks += 1e9; return ticks, nil })
	}
	head, err := f.rule.Head()
	depthMust(t, err)
	recursive, err := f.manager.NewRule(head, []ast.ASTHandle{head}, []bool{false}, "timed-recursion")
	depthMust(t, err)
	defer func() { depthMust(t, recursive.Close()) }()
	rules, err := base.NewRuleSet(f.manager)
	depthMust(t, err)
	defer func() { depthMust(t, rules.Close()) }()
	depthMust(t, rules.Add(f.rule))
	depthMust(t, rules.Add(recursive))
	depthMust(t, c.UpdateRules(rules))
	status, err := c.Solve(0, 4)
	depthMust(t, err)
	if status != lbool.True {
		t.Fatal("expected reachable initial fact", status)
	}
	p := c.transformers[f.query]
	truth, err := m.MkTrue()
	depthMust(t, err)
	obligation, err := p.NewProofObligation(nil, 2, 1, truth, nil)
	depthMust(t, err)
	md, err := model.NewModel(m)
	depthMust(t, err)
	defer func() { depthMust(t, md.Close()) }()
	last, found, err := p.LastReachCaseVariable()
	depthMust(t, err)
	if !found {
		t.Fatal("missing initial reach fact")
	}
	origin, err := c.owner.FormulaNewToOld(last, 0, false)
	depthMust(t, err)
	defer func() { depthMust(t, m.DelRef(&origin)) }()
	originHandle, err := m.RefGet(origin)
	depthMust(t, err)
	declaration, err := m.GetDecl(originHandle)
	depthMust(t, err)
	falsehood, err := m.MkFalse()
	depthMust(t, err)
	depthMust(t, md.Core().RegisterConstant(declaration, falsehood))
	child, err := p.CreateDerivationChild(obligation, recursive, md.Core(), []bool{false}, false)
	depthMust(t, err)
	if child == nil {
		t.Fatal("expected derivation child")
	}
	confirmed, err := c.queue.confirmConcreteReachability(child)
	depthMust(t, err)
	if !confirmed {
		t.Fatal("initial state was not concretely reachable")
	}
	_, err = c.propagate(InfinityLevel, 0)
	depthMust(t, err)
	keys := []statistics.Key{"time.spacer.init_rules", "time.spacer.solve", "time.spacer.solve.propagate", "time.spacer.solve.reach", "time.spacer.solve.reach.is-reach", "time.spacer.solve.reach.children"}
	values := func(stats *statistics.Statistics) map[statistics.Key]float64 {
		result := make(map[statistics.Key]float64)
		for i := 0; i < stats.Size(); i++ {
			if !stats.IsUint(i) {
				result[stats.Key(i)] += stats.DoubleValue(i)
			}
		}
		return result
	}
	check := func(stats *statistics.Statistics, expected float64) {
		t.Helper()
		got := values(stats)
		for _, key := range keys {
			if got[key] != expected {
				t.Fatal("context timer", key, got[key], expected)
			}
		}
	}
	stats := statistics.New()
	depthMust(t, c.CollectStatistics(stats))
	check(stats, 1)
	snapshot, err := c.SnapshotStatistics()
	depthMust(t, err)
	depthMust(t, c.Reset())
	stats = statistics.New()
	depthMust(t, c.CollectStatistics(stats))
	check(stats, 1)
	stats = statistics.New()
	depthMust(t, snapshot.Collect(stats))
	check(stats, 1)
	snapshot.Reset()
	stats = statistics.New()
	depthMust(t, snapshot.Collect(stats))
	check(stats, 0)
	depthMust(t, c.ResetStatistics())
	stats = statistics.New()
	depthMust(t, c.CollectStatistics(stats))
	check(stats, 0)
}
