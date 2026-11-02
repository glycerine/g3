package duality

import "testing"

func TestUnwindingUpperUpdateInvalidatesCoveringsOnlyForNewFacts(t *testing.T) {
	ctx, _, aux := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	term, err := ctx.BoolConst("outer-update-R")
	must(err)
	closeExpr(t, term)
	relation, err := source.CreateNode(term)
	must(err)
	graph, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	defer graph.Close()
	covering, err := NewCovering(graph, aux)
	must(err)
	unwinding, err := NewUnwinding(source, graph, covering)
	must(err)
	older, err := unwinding.CreateNodeInstance(relation, 0)
	must(err)
	later, err := unwinding.CreateNodeInstance(relation, 0)
	must(err)
	must(unwinding.ActivateInstance(older))
	must(unwinding.ActivateInstance(later))
	p, err := ctx.BoolConst("outer-update-p")
	must(err)
	closeExpr(t, p)
	q, err := ctx.BoolConst("outer-update-q")
	must(err)
	closeExpr(t, q)
	fact, err := NewTransformer(ctx, nil, nil, p)
	must(err)
	defer fact.Close()
	must(replaceNodeAnnotation(older, fact))
	must(replaceNodeAnnotation(later, fact))
	_, err = graph.CreateLowerBoundEdge(older)
	must(err)
	_, err = graph.CreateLowerBoundEdge(later)
	must(err)
	covered, err := covering.CoverByNode(later, older)
	must(err)
	if !covered {
		t.Fatal("fixture covering not established")
	}
	conjunction, err := p.And(q)
	must(err)
	closeExpr(t, conjunction)
	stronger, err := NewTransformer(ctx, nil, nil, conjunction)
	must(err)
	defer stronger.Close()
	changed, err := unwinding.Update(older, stronger, aux)
	must(err)
	if !changed || !unwinding.updatedNodes[relation] || !covering.someUpdates {
		t.Fatal("new fact not tracked")
	}
	stillCovered, err := covering.IsCovered(later)
	must(err)
	if stillCovered {
		t.Fatal("old covering survived strengthened fact")
	}
	forward, err := older.Annotation.SubsetEq(stronger, aux)
	must(err)
	backward, err := stronger.SubsetEq(older.Annotation, aux)
	must(err)
	if !forward || !backward {
		t.Fatal("upper fact intersection changed meaning")
	}
	unwinding.updatedNodes = nil
	covering.someUpdates = false
	changed, err = unwinding.Update(older, stronger, aux)
	must(err)
	if changed || len(unwinding.updatedNodes) != 0 || covering.someUpdates {
		t.Fatal("redundant update invalidated/tracked upper facts")
	}
	full, err := relation.Annotation.IsFull()
	must(err)
	if !full {
		t.Fatal("upper update changed original rule annotation")
	}
}

func TestUnwindingPublishesSharedChildUpperAndLowerFactsInPositionOrder(t *testing.T) {
	ctx, _, aux := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	r, err := ctx.BoolConst("publish-R")
	must(err)
	closeExpr(t, r)
	sourceParent, err := source.CreateNode(r)
	must(err)
	sourceChild, err := source.CreateNode(r)
	must(err)
	sourceEdge, err := source.CreateEdge(sourceParent, sourceParent.Annotation, []*Node{sourceChild, sourceChild})
	must(err)
	graph, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	defer graph.Close()
	covering, err := NewCovering(graph, aux)
	must(err)
	unwinding, err := NewUnwinding(source, graph, covering)
	must(err)
	parent, err := unwinding.CreateNodeInstance(sourceParent, 0)
	must(err)
	child, err := unwinding.CreateNodeInstance(sourceChild, 0)
	must(err)
	_, err = unwinding.CreateEdgeInstance(sourceEdge, parent, []*Node{child, child})
	must(err)
	derived, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	defer derived.Close()
	top, err := derived.CloneNode(parent)
	must(err)
	first, err := derived.CloneNode(child)
	must(err)
	second, err := derived.CloneNode(child)
	must(err)
	_, err = derived.CreateEdge(top, parent.Outgoing.F, []*Node{first, second})
	must(err)
	p, err := ctx.BoolConst("publish-p")
	must(err)
	closeExpr(t, p)
	q, err := ctx.BoolConst("publish-q")
	must(err)
	closeExpr(t, q)
	pFact, err := NewTransformer(ctx, nil, nil, p)
	must(err)
	defer pFact.Close()
	qFact, err := NewTransformer(ctx, nil, nil, q)
	must(err)
	defer qFact.Close()
	must(replaceNodeAnnotation(top, pFact))
	must(replaceNodeAnnotation(first, pFact))
	must(replaceNodeAnnotation(second, qFact))
	heuristic, err := NewExpansionHeuristic(graph)
	must(err)
	must(unwinding.UpdateWithInterpolant(parent, top, aux, heuristic))
	conjunction, err := p.And(q)
	must(err)
	closeExpr(t, conjunction)
	upper, err := NewTransformer(ctx, nil, nil, conjunction)
	must(err)
	defer upper.Close()
	forward, err := child.Annotation.SubsetEq(upper, aux)
	must(err)
	backward, err := upper.SubsetEq(child.Annotation, aux)
	must(err)
	if !forward || !backward || heuristic.updates[child] != 2 || heuristic.updates[parent] != 1 {
		t.Fatal("shared upper facts not intersected per child position")
	}
	must(unwinding.UpdateWithInterpolant(parent, top, aux, heuristic))
	if heuristic.updates[child] != 4 || heuristic.updates[parent] != 2 {
		t.Fatal("redundant upper facts did not update source heuristic")
	}
	unwinding.updatedNodes = nil
	covering.someUpdates = false
	for _, pair := range []struct {
		node *Node
		fact *Transformer
	}{{top, pFact}, {first, pFact}, {second, qFact}} {
		must(pair.node.Underapprox.Close())
		pair.node.Underapprox, err = pair.fact.Clone()
		must(err)
	}
	must(unwinding.UpdateWithCounterexample(parent, top, aux, heuristic))
	disjunction, err := p.Or(q)
	must(err)
	closeExpr(t, disjunction)
	lower, err := NewTransformer(ctx, nil, nil, disjunction)
	must(err)
	defer lower.Close()
	forward, err = child.Underapprox.SubsetEq(lower, aux)
	must(err)
	backward, err = lower.SubsetEq(child.Underapprox, aux)
	must(err)
	if !forward || !backward || heuristic.updates[child] != 6 || heuristic.updates[parent] != 3 {
		t.Fatal("shared lower facts not unioned per child position")
	}
	if len(unwinding.updatedNodes) != 0 || covering.someUpdates {
		t.Fatal("lower facts invalidated upper-fact covering state")
	}
	must(unwinding.UpdateWithCounterexample(parent, top, aux, heuristic))
	if heuristic.updates[child] != 6 || heuristic.updates[parent] != 3 {
		t.Fatal("redundant lower facts updated heuristic")
	}
}
