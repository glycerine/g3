package duality

import "testing"

func TestDualityLearnFromKeepsReplayGuideUntilHistoryIsOwned(t *testing.T) {
	context, _, auxiliary := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	oldSource, err := NewRPFP(context)
	must(err)
	defer func() { must(oldSource.Close()) }()
	newSource, err := NewRPFP(context)
	must(err)
	defer func() { must(newSource.Close()) }()
	oldGraph, err := NewRPFPWithVocabulary(oldSource.vocabulary)
	must(err)
	defer func() { must(oldGraph.Close()) }()

	oldX, err := context.BoolConst("learn-x@@old")
	must(err)
	closeExpr(t, oldX)
	newX, err := context.BoolConst("learn-x@@new")
	must(err)
	closeExpr(t, newX)
	oldBoolean, err := oldX.Sort()
	must(err)
	defer func() { must(oldBoolean.Close()) }()
	newBoolean, err := newX.Sort()
	must(err)
	defer func() { must(newBoolean.Close()) }()
	makeNode := func(context *Context, graph *RPFP, name string, parameter *Expr, boolean *Sort) *Node {
		t.Helper()
		symbol, err := context.StringSymbol(name)
		must(err)
		declaration, err := context.Function(symbol, []*Sort{boolean}, boolean)
		must(err)
		defer func() { must(declaration.Close()) }()
		term, err := declaration.Call(parameter)
		must(err)
		defer func() { must(term.Close()) }()
		node, err := graph.CreateNode(term)
		must(err)
		return node
	}
	oldNode := makeNode(context, oldGraph, "learn-R@@old", oldX, oldBoolean)
	oldFormula, err := oldX.Not()
	must(err)
	closeExpr(t, oldFormula)
	oldFact, err := NewTransformer(context, nil, []*Expr{oldX}, oldFormula)
	must(err)
	must(replaceNodeAnnotation(oldNode, oldFact))
	must(oldFact.Close())
	_ = makeNode(context, newSource, "learn-R@@new", newX, newBoolean)

	replayGraph, err := NewRPFPWithVocabulary(oldSource.vocabulary)
	must(err)
	replayTerm, err := context.BoolConst("learn-from-transaction-replay")
	must(err)
	closeExpr(t, replayTerm)
	replayRoot, err := replayGraph.CreateNode(replayTerm)
	must(err)
	refusingHistory, err := NewRPFP(context)
	must(err)
	must(refusingHistory.Close())

	oldEngine := &Engine{source: oldSource, graph: refusingHistory, counterexample: NewCounterexample()}
	oldEngine.self = oldEngine
	defer func() { must(oldEngine.Close()) }()
	newEngine := &Engine{source: newSource, counterexample: NewCounterexample(), config: EngineConfig{Auxiliary: auxiliary}}
	newEngine.self = newEngine
	defer func() { must(newEngine.Close()) }()
	must(oldEngine.counterexample.Set(replayGraph, replayRoot))

	if failure := newEngine.LearnFrom(oldEngine); failure == nil {
		t.Fatal("closed history graph unexpectedly accepted")
	}
	if oldEngine.counterexample.Tree() != replayGraph || oldEngine.counterexample.Root() != replayRoot || replayGraph.closed {
		t.Fatal("failed history acquisition lost the old replay guide")
	}
	if newEngine.counterexample.Tree() != nil || len(newEngine.proposers) != 0 {
		t.Fatal("failed history acquisition partially mutated the new engine")
	}
	oldEngine.graph = oldGraph
	must(newEngine.LearnFrom(oldEngine))
	if oldEngine.counterexample.Tree() != nil || newEngine.counterexample.Tree() != replayGraph || newEngine.counterexample.Root() != replayRoot {
		t.Fatal("successful LearnFrom did not transfer the exact replay guide")
	}
	if len(newEngine.proposers) != 1 || replayGraph.closed {
		t.Fatal("successful LearnFrom did not publish both replay and history guidance")
	}
	_, failure := newEngine.Solve()
	if failure == nil {
		t.Fatal("incomplete learned-solve services unexpectedly succeeded")
	}
	if newEngine.counterexample.Tree() != replayGraph || newEngine.counterexample.Root() != replayRoot || newEngine.heuristic != nil {
		t.Fatal("failed solve did not restore the replay guide before service cleanup", failure)
	}
	must(newEngine.RollbackLearnFrom(oldEngine))
	if oldEngine.counterexample.Tree() != replayGraph || oldEngine.counterexample.Root() != replayRoot || newEngine.counterexample.Tree() != nil {
		t.Fatal("failed-run rollback did not return the exact replay guide")
	}
	must(newEngine.closeProposers())
	must(newEngine.LearnFrom(oldEngine))
	if oldEngine.counterexample.Tree() != nil || newEngine.counterexample.Tree() != replayGraph || len(newEngine.proposers) != 1 {
		t.Fatal("retry did not relearn replay and history guidance")
	}
}
