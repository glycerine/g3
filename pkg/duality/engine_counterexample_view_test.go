package duality

import "testing"

func TestDualityInvalidateCounterexampleViewPreservesReplayTransfer(t *testing.T) {
	ctx, _, _ := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	oldSource, err := NewRPFP(ctx)
	must(err)
	defer func() { must(oldSource.Close()) }()
	newSource, err := NewRPFP(ctx)
	must(err)
	defer func() { must(newSource.Close()) }()

	// LearnFrom also snapshots annotations from the completed old unwinding.
	oldGraph, err := NewRPFPWithVocabulary(oldSource.vocabulary)
	must(err)
	replayGraph, err := NewRPFPWithVocabulary(oldSource.vocabulary)
	must(err)
	replayTerm, err := ctx.BoolConst("counterexample-view-replay")
	must(err)
	closeExpr(t, replayTerm)
	replayRoot, err := replayGraph.CreateNode(replayTerm)
	must(err)

	oldEngine := &Engine{source: oldSource, graph: oldGraph, counterexample: NewCounterexample()}
	oldEngine.self = oldEngine
	defer func() { must(oldEngine.Close()) }()
	newEngine := &Engine{source: newSource, counterexample: NewCounterexample()}
	newEngine.self = newEngine
	defer func() { must(newEngine.Close()) }()

	must(oldEngine.counterexample.Set(replayGraph, replayRoot))
	borrowed := oldEngine.GetCounterexample()
	must(oldEngine.InvalidateCounterexampleView())
	if borrowed.Tree() != nil || borrowed.Root() != nil {
		t.Fatal("counterexample rotation left the prior borrowed view live")
	}
	if oldEngine.counterexample == borrowed || oldEngine.counterexample.Tree() != replayGraph || replayGraph.closed {
		t.Fatal("counterexample rotation lost its replay derivation owner")
	}

	must(newEngine.LearnFrom(oldEngine))
	if oldEngine.counterexample.Tree() != nil || newEngine.counterexample.Tree() != replayGraph || newEngine.counterexample.Root() != replayRoot {
		t.Fatal("LearnFrom did not transfer the preserved replay derivation")
	}
	if len(newEngine.proposers) != 1 || replayGraph.closed {
		t.Fatal("LearnFrom did not preserve both replay and history guidance")
	}
}
