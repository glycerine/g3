package duality

import "testing"

func TestUnwindingSourcePostorderHandlesCyclesAndOpenNodes(t *testing.T) {
	ctx, _, aux := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	term, err := ctx.BoolConst("ordered-R")
	must(err)
	closeExpr(t, term)
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	a, err := source.CreateNode(term)
	must(err)
	b, err := source.CreateNode(term)
	must(err)
	open, err := source.CreateNode(term)
	must(err)
	disconnected, err := source.CreateNode(term)
	must(err)
	ae, err := source.CreateEdge(a, a.Annotation, []*Node{b, b})
	must(err)
	be, err := source.CreateEdge(b, b.Annotation, []*Node{a})
	must(err)
	de, err := source.CreateLowerBoundEdge(disconnected)
	must(err)
	graph, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	defer graph.Close()
	covering, err := NewCovering(graph, aux)
	must(err)
	u, err := NewUnwinding(source, graph, covering)
	must(err)
	must(u.DoTopoSort())
	if len(u.topological) != 4 || u.topological[b] != 0 || u.topological[a] != 1 || u.topological[open] != 2 || u.topological[disconnected] != 3 {
		t.Fatal("source postorder ranks", u.topological)
	}
	expected := []*Edge{be, ae, nil, de}
	if len(u.sortedEdges) != len(expected) {
		t.Fatal("sorted edge count")
	}
	for n, edge := range expected {
		if u.sortedEdges[n] != edge {
			t.Fatal("source sorted-edge order", n)
		}
	}
	// Recursion counting consumes the computed source ranks directly.
	child, err := u.CreateNodeInstance(b, 0)
	must(err)
	_, err = graph.CreateLowerBoundEdge(child)
	must(err)
	parent, err := u.CreateNodeInstance(a, 0)
	must(err)
	_, err = u.CreateEdgeInstance(ae, parent, []*Node{child, child})
	must(err)
	must(u.UpdateBackEdges(parent, u.topological))
	if len(u.backEdges[parent]) != 0 {
		t.Fatal("descending source edge counted as recursion")
	}
	back, err := u.CreateNodeInstance(b, 0)
	must(err)
	_, err = u.CreateEdgeInstance(be, back, []*Node{parent})
	must(err)
	must(u.UpdateBackEdges(back, u.topological))
	if u.backEdges[back][a] != 1 {
		t.Fatal("back edge not counted")
	}
	// Recomputing clears stale ranks and follows the current single outgoing
	// edge, even though the source graph still owns older edge objects.
	replacement, err := source.CreateLowerBoundEdge(b)
	must(err)
	must(u.DoTopoSort())
	if len(u.sortedEdges) != 4 || u.sortedEdges[0] != replacement || u.topological[b] != 0 || u.topological[a] != 1 {
		t.Fatal("ordering did not refresh")
	}
}
