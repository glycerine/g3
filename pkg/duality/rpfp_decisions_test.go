package duality

import (
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/statistics"
	"testing"
)

func TestDualityCumulativeDecisionsNativeScopes(t *testing.T) {
	ctx, _, native := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	graph, err := NewRPFP(ctx)
	must(err)
	defer graph.Close()
	if _, err = graph.CumulativeDecisions(); err == nil {
		t.Fatal("accepted unbound solver")
	}
	must(graph.BindSolver(native))
	before, err := graph.CumulativeDecisions()
	must(err)
	if before != 0 {
		t.Fatal("fresh solver decisions", before)
	}
	a, err := ctx.BoolConst("decision-a")
	must(err)
	closeExpr(t, a)
	b, err := ctx.BoolConst("decision-b")
	must(err)
	closeExpr(t, b)
	disjunction, err := a.Or(b)
	must(err)
	closeExpr(t, disjunction)
	h, err := disjunction.Raw()
	must(err)
	must(graph.Push())
	must(native.Assert(h))
	status, err := graph.Check(nil, nil, nil)
	must(err)
	if status != lbool.True {
		t.Fatal(status)
	}
	count, err := graph.CumulativeDecisions()
	must(err)
	if count <= before {
		t.Fatal("decision-requiring native query did not advance counter", count, before)
	}
	stats := statistics.New()
	must(native.CollectStatistics(stats))
	var expected uint32
	for i := 0; i < stats.Size(); i++ {
		if stats.Key(i) == "decisions" && stats.IsUint(i) {
			expected += stats.UintValue(i)
		}
	}
	if count != int(expected) {
		t.Fatal("counter differs from native statistics", count, expected)
	}
	must(graph.Pop(1))
	after, err := graph.CumulativeDecisions()
	must(err)
	if after != count {
		t.Fatal("scope pop reset cumulative decisions", after, count)
	}
	must(graph.Close())
	if _, err = graph.CumulativeDecisions(); err == nil {
		t.Fatal("accepted closed graph")
	}
}
