package duality

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/lbool"
	"testing"
)

func TestDualityReplayHeuristicNativeMatchingPriorityAndOwnership(t *testing.T) {
	ctx, m, native, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	old, err := NewRPFP(ctx)
	must(err)
	must(old.BindSolver(native))
	must(old.BindModelEvaluator(derivationEvaluatorFixture(t, ctx, globals)))
	newGraph, err := NewRPFP(ctx)
	must(err)
	defer newGraph.Close()
	node := func(graph *RPFP, name string) *Node {
		t.Helper()
		term, err := ctx.BoolConst(name)
		must(err)
		closeExpr(t, term)
		result, err := graph.CreateNode(term)
		must(err)
		return result
	}
	oldRoot := node(old, "replay-old-root")
	oldA := node(old, "replay-A@@old")
	oldB := node(old, "replay-B@@old")
	_, err = old.CreateEdge(oldRoot, oldRoot.Annotation, []*Node{oldA, oldB})
	must(err)
	must(old.Push())
	for _, item := range []struct {
		node   *Node
		active bool
	}{{oldRoot, true}, {oldA, true}, {oldB, false}} {
		activation, vars, err := old.RedVars(item.node)
		must(err)
		defer activation.Close()
		for _, variable := range vars {
			defer variable.Close()
		}
		raw, err := activation.Raw()
		must(err)
		if !item.active {
			raw, err = m.MkNot(raw)
			must(err)
		}
		must(native.Assert(raw))
	}
	status, err := old.Check(nil, nil, nil)
	must(err)
	if status != lbool.True {
		t.Fatal(status)
	}
	must(old.Pop(1))
	cex := NewCounterexample()
	defer cex.Close()
	must(cex.Set(old, oldRoot))
	heuristic, err := NewReplayExpansionHeuristic(newGraph, cex)
	must(err)
	defer heuristic.Close()
	if cex.Tree() != nil || old.closed {
		t.Fatal("replay did not take exclusive old graph ownership")
	}
	root := node(newGraph, "replay-new-root")
	unmatched := node(newGraph, "replay-new-inserted")
	a := node(newGraph, "replay-A@@new")
	b := node(newGraph, "replay-B@@new")
	extra := node(newGraph, "replay-extra")
	_, err = newGraph.CreateEdge(root, root.Annotation, []*Node{unmatched, a, b, extra})
	must(err)
	for _, n := range []*Node{root, unmatched, a, b, extra} {
		n.Map = n
	}
	var best []*Node
	must(heuristic.ChooseExpand([]*Node{root}, &best, true, true))
	if len(best) != 1 || best[0] != root {
		t.Fatal("replay root was not seeded/high-priority expanded")
	}
	best = nil
	must(heuristic.ChooseExpand([]*Node{unmatched, a, b, extra}, &best, true, false))
	if len(best) != 1 || best[0] != a || heuristic.replay.matches[a] != oldA || heuristic.replay.matches[b] != oldB || heuristic.replay.matches[unmatched] != nil || heuristic.replay.matches[extra] != nil {
		t.Fatal("greedy suffix/name matching or old-model activity filtering")
	}
	// Replay's ordinary fallback drops best_only and uses the midpoint cutoff.
	heuristic.updates[b] = 4
	heuristic.updates[extra] = 2
	best = nil
	must(heuristic.ChooseExpand([]*Node{a, b, extra}, &best, false, true))
	if len(best) != 2 || best[0] != a || best[1] != extra {
		t.Fatal("source replay fallback unexpectedly honored best_only")
	}
	orphan := node(newGraph, "replay-orphan")
	if err = heuristic.ChooseExpand([]*Node{orphan}, &best, true, false); !errors.Is(err, ErrReplayMapping) {
		t.Fatal("unmatched-parent diagnostic missing", err)
	}
	must(heuristic.Done())
	if !old.closed || heuristic.replay.previous.Tree() != nil || len(heuristic.replay.matches) != 0 || newGraph.closed {
		t.Fatal("Done did not release only the replay graph")
	}
	// Once the old graph is gone, even high-priority replay calls use ordinary
	// base selection (the source passes false to the base class).
	best = nil
	must(heuristic.ChooseExpand([]*Node{a, b, extra}, &best, true, true))
	if len(best) != 2 || best[0] != a || best[1] != extra {
		t.Fatal("post-Done replay fallback changed")
	}
	empty := []*Node(nil)
	must(heuristic.ChooseExpand(empty, &best, true, false))
}

func TestDualityReplayBaseNamesAndInterruptedCleanup(t *testing.T) {
	ctx, _, _ := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	old, err := NewRPFP(ctx)
	must(err)
	target, err := NewRPFP(ctx)
	must(err)
	defer target.Close()
	var root *Node
	for _, test := range []struct{ name, want string }{{"R@@suffix", "R"}, {"@@literal", "@@literal"}, {"R", "R"}} {
		term, err := ctx.BoolConst(test.name)
		must(err)
		closeExpr(t, term)
		n, err := old.CreateNode(term)
		must(err)
		root = n
		got, err := replayNodeBaseName(n)
		must(err)
		if got != test.want {
			t.Fatal(got, test.want)
		}
	}
	owner := NewCounterexample()
	defer owner.Close()
	must(owner.Set(old, root))
	h, err := NewReplayExpansionHeuristic(target, owner)
	must(err)
	must(h.Close())
	must(h.Close())
	if !old.closed || owner.Tree() != nil {
		t.Fatal("interrupted replay owner leaked")
	}
}
