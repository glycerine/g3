package duality

import "testing"

func TestDualityInterruptedReplayRestoresGuideAndDiscardsPartialResult(t *testing.T) {
	ctx, _, _ := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	graphWithRoot := func(name string) (*RPFP, *Node) {
		t.Helper()
		graph, err := NewRPFP(ctx)
		must(err)
		term, err := ctx.BoolConst(name)
		must(err)
		defer func() { must(term.Close()) }()
		root, err := graph.CreateNode(term)
		must(err)
		return graph, root
	}

	current, _ := graphWithRoot("replay-rollback-current")
	defer func() { must(current.Close()) }()
	guideGraph, guideRoot := graphWithRoot("replay-rollback-guide")
	partialGraph, partialRoot := graphWithRoot("replay-rollback-partial")

	guide := NewCounterexample()
	defer func() { must(guide.Close()) }()
	must(guide.Set(guideGraph, guideRoot))
	heuristic, err := NewReplayExpansionHeuristic(current, guide)
	must(err)
	partial := NewCounterexample()
	defer func() { must(partial.Close()) }()
	must(partial.Set(partialGraph, partialRoot))

	must(heuristic.restoreReplay(partial))
	if partial.Tree() != guideGraph || partial.Root() != guideRoot || heuristic.replay.previous.Tree() != partialGraph {
		t.Fatal("interrupted replay did not exchange guide and partial result owners")
	}
	must(heuristic.Close())
	if guideGraph.closed || !partialGraph.closed || partial.Tree() != guideGraph {
		t.Fatal("replay cleanup did not preserve the guide and discard the partial result")
	}
}
