package spacer

import (
	"errors"
	"testing"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/statistics"
	"github.com/glycerine/g3/pkg/util/stopwatch"
)

func spacerTimingValue(t *testing.T, p *PredicateTransformer, key statistics.Key) float64 {
	t.Helper()
	stats := statistics.New()
	depthMust(t, p.CollectStatistics(stats))
	var result float64
	for i := 0; i < stats.Size(); i++ {
		if !stats.IsUint(i) && stats.Key(i) == key {
			result += stats.DoubleValue(i)
		}
	}
	return result
}

func TestSpacerPredicateSourceTimerScopesAndReset(t *testing.T) {
	f := newContextAnswerFixture(t, false)
	p, err := NewRulePredicateTransformer(f.context.owner, f.query, f.context.config)
	depthMust(t, err)
	defer func() { depthMust(t, p.Close()) }()
	var ticks uint64
	clock := func() (uint64, error) { ticks += 1e9; return ticks, nil }
	p.initializeWatch = stopwatch.New(clock)
	p.mustReachableWatch = stopwatch.New(clock)
	depthMust(t, p.InitializeRules(map[ast.ASTHandle]*PredicateTransformer{f.query: p}, []*base.Rule{f.rule}))
	if got := spacerTimingValue(t, p, "time.spacer.init_rules.pt.init"); got != 1 {
		t.Fatal("rule timer scope", got)
	}
	truth, err := p.manager.MkTrue()
	depthMust(t, err)
	reachable, md, err := p.IsMustReachable(truth, false)
	depthMust(t, err)
	if reachable || md != nil {
		t.Fatal("fixture already has reach facts")
	}
	depthMust(t, p.InitReachFacts())
	reachable, md, err = p.IsMustReachable(truth, true)
	depthMust(t, err)
	if !reachable || md == nil {
		t.Fatal("expected concrete reachability model", reachable)
	}
	depthMust(t, md.DecRef())
	if got := spacerTimingValue(t, p, "time.spacer.solve.pt.must_reachable"); got != 2 {
		t.Fatal("early-return and solver timer accumulation", got)
	}
	if p.initializeWatch.Running() || p.mustReachableWatch.Running() {
		t.Fatal("scope left timer running")
	}
	depthMust(t, p.ResetStatistics())
	for _, key := range []statistics.Key{"time.spacer.init_rules.pt.init", "time.spacer.solve.pt.must_reachable"} {
		if got := spacerTimingValue(t, p, key); got != 0 {
			t.Fatal("statistics reset left transformer timer", key, got)
		}
	}
	clockFailure := errors.New("test clock stop failure")
	reads := 0
	p.mustReachableWatch = stopwatch.New(func() (uint64, error) {
		reads++
		if reads == 2 {
			return 0, clockFailure
		}
		return uint64(reads) * 1e9, nil
	})
	_, md, err = p.IsMustReachable(truth, true)
	if !errors.Is(err, clockFailure) || md != nil {
		t.Fatal("timer-close error published an owning model", md, err)
	}
	depthMust(t, p.mustReachableWatch.Stop())
}

func TestSpacerRuleTimerFailureLeavesInitializationRetryable(t *testing.T) {
	f := newContextAnswerFixture(t, false)
	p, err := NewRulePredicateTransformer(f.context.owner, f.query, f.context.config)
	depthMust(t, err)
	defer func() { depthMust(t, p.Close()) }()
	clockFailure := errors.New("test rule timer stop failure")
	reads := 0
	p.initializeWatch = stopwatch.New(func() (uint64, error) {
		reads++
		if reads == 2 {
			return 0, clockFailure
		}
		return uint64(reads) * 1e9, nil
	})
	transformers := map[ast.ASTHandle]*PredicateTransformer{f.query: p}
	err = p.InitializeRules(transformers, []*base.Rule{f.rule})
	if !errors.Is(err, clockFailure) || p.initialized || len(p.rules) != 0 {
		t.Fatal("rule timer failure published initialized rules", err)
	}
	depthMust(t, p.initializeWatch.Stop())
	depthMust(t, p.InitializeRules(transformers, []*base.Rule{f.rule}))
	if !p.initialized || len(p.rules) != 1 {
		t.Fatal("rule initialization did not recover")
	}
}
