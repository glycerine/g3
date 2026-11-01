package duality

import "testing"

func TestUnwindingInstanceQueueActivationAndSourceEdgeMaps(t *testing.T) {
	ctx, _, aux := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	term, err := ctx.BoolConst("unwinding-R")
	must(err)
	closeExpr(t, term)
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	relation, err := source.CreateNode(term)
	must(err)
	must(relation.Annotation.SetEmpty())
	edge, err := source.CreateLowerBoundEdge(relation)
	must(err)
	graph, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	defer graph.Close()
	covering, err := NewCovering(graph, aux)
	must(err)
	unwinding, err := NewUnwinding(source, graph, covering)
	must(err)
	first, err := unwinding.CreateNodeInstance(relation, 500)
	must(err)
	negative, err := unwinding.CreateNodeInstance(relation, -3)
	must(err)
	second, err := unwinding.CreateNodeInstance(relation, 0)
	must(err)
	if first.Number != 1 || second.Number != 3 || negative.Number != -3 {
		t.Fatal("source numbering override")
	}
	for _, node := range []*Node{first, negative, second} {
		full, err := node.Annotation.IsFull()
		must(err)
		if !full || node.Map != relation {
			t.Fatal("instance initialization")
		}
	}
	pending := unwinding.PendingNodes()
	if len(pending) != 3 || pending[0] != negative || pending[1] != first || pending[2] != second {
		t.Fatal("source numbered expansion order")
	}
	pending[0] = second
	if unwinding.PendingNodes()[0] != negative {
		t.Fatal("pending slice aliases search state")
	}
	if len(covering.instances[relation]) != 0 || len(covering.allOfNode[relation]) != 3 {
		t.Fatal("unexpanded instances incorrectly active")
	}
	must(unwinding.ActivateInstance(first))
	if len(unwinding.PendingNodes()) != 2 || len(covering.instances[relation]) != 1 || covering.instances[relation][0] != first {
		t.Fatal("activation state")
	}
	created, err := unwinding.CreateEdgeInstance(edge, first, nil)
	must(err)
	if created.Map != edge || created.Parent != first {
		t.Fatal("unwinding edge source map")
	}
	// Same-number queue keys follow source comparator equivalence while both
	// allocated instances remain in the all-instance invalidation history.
	duplicate, err := unwinding.CreateNodeInstance(relation, -3)
	must(err)
	if duplicate == negative || len(unwinding.PendingNodes()) != 2 || len(covering.allOfNode[relation]) != 4 {
		t.Fatal("equal-number source set behavior")
	}
	must(unwinding.ActivateInstance(negative))
	must(unwinding.ActivateInstance(second))
	if len(unwinding.PendingNodes()) != 0 || len(covering.instances[relation]) != 3 {
		t.Fatal("queue did not drain")
	}
}
