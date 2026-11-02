package duality

import "testing"

func TestRPFPRemoveDeadNodesAllClausesCyclesAndRepeatedChildren(t *testing.T) {
	ctx, _, _ := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	g, err := NewRPFP(ctx)
	must(err)
	defer g.Close()
	create := func(name string) *Node {
		term, err := ctx.BoolConst(name)
		must(err)
		defer term.Close()
		node, err := g.CreateNode(term)
		must(err)
		return node
	}
	dead := create("dead")
	root := create("query")
	a := create("a")
	b := create("b")
	cycle := create("cycle")
	island := create("island")
	must(root.Bound.SetEmpty())
	edge := func(parent *Node, children ...*Node) *Edge {
		e, err := g.CreateEdge(parent, parent.Annotation, children)
		must(err)
		return e
	}
	deadEdge := edge(dead, a, a)
	first := edge(root, a)
	second := edge(root, b) // Outgoing only points to this clause.
	recursive := edge(a, a)
	leaf := edge(b)
	cycleEdge := edge(cycle, island)
	islandEdge := edge(island, cycle)
	if len(a.Incoming) != 4 {
		t.Fatal("fixture repeated incoming count")
	}
	must(g.RemoveDeadNodes())
	if len(g.nodes) != 3 || g.nodes[0] != root || g.nodes[1] != a || g.nodes[2] != b {
		t.Fatal("live source node order mismatch")
	}
	for _, node := range []*Node{dead, cycle, island} {
		if !node.closed {
			t.Fatal("dead node still open")
		}
	}
	for _, e := range []*Edge{deadEdge, cycleEdge, islandEdge} {
		if !e.closed {
			t.Fatal("dead edge still open")
		}
	}
	wants := []*Edge{first, second, recursive, leaf}
	if len(g.edges) != len(wants) {
		t.Fatal("wrong edge count")
	}
	for i, e := range wants {
		if g.edges[i] != e || e.closed {
			t.Fatal("live clause order changed")
		}
	}
	if len(a.Incoming) != 2 || a.Incoming[0] != first || a.Incoming[1] != recursive {
		t.Fatal("dead repeated incoming references survived")
	}
	if root.Outgoing != second {
		t.Fatal("live outgoing pointer changed")
	}
	must(g.RemoveDeadNodes())
	if len(g.nodes) != 3 || len(g.edges) != 4 {
		t.Fatal("second pruning changed live graph")
	}
	// Without a constrained root even a recursive component has no consumer.
	must(root.Bound.SetFull())
	must(g.RemoveDeadNodes())
	if len(g.nodes) != 0 || len(g.edges) != 0 {
		t.Fatal("unqueried graph survived")
	}
	must(g.RemoveDeadNodes())
	// Allocation numbers remain monotone after collection.
	next := create("next")
	if next.Number != 7 {
		t.Fatal("pruning reset node numbering")
	}
}
