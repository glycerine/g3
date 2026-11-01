package duality

import "testing"

func TestUnwindingCandidatesCartesianOrderAndStratifiedDuplicate(t *testing.T) {
	ctx, _, aux := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	term, err := ctx.BoolConst("candidate-R")
	must(err)
	closeExpr(t, term)
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	relation, err := source.CreateNode(term)
	must(err)
	edge, err := source.CreateEdge(relation, relation.Annotation, []*Node{relation, relation})
	must(err)
	nullary, err := source.CreateLowerBoundEdge(relation)
	must(err)
	graph, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	defer graph.Close()
	covering, err := NewCovering(graph, aux)
	must(err)
	u, err := NewUnwinding(source, graph, covering)
	must(err)
	a, err := u.CreateNodeInstance(relation, 0)
	must(err)
	b, err := u.CreateNodeInstance(relation, 0)
	must(err)
	must(u.GenCandidates(99, edge, [][]*Node{{a, b}, {b, a}}))
	expected := [][2]*Node{{a, b}, {a, a}, {b, b}, {b, a}}
	for _, pair := range expected {
		candidate, ok := u.NextCandidate()
		if !ok || candidate.Edge != edge || len(candidate.Children) != 2 || candidate.Children[0] != pair[0] || candidate.Children[1] != pair[1] {
			t.Fatal("Cartesian order", candidate)
		}
	}
	if _, ok := u.NextCandidate(); ok {
		t.Fatal("extra product candidate")
	}
	must(u.GenCandidates(0, edge, [][]*Node{{a}, nil}))
	if _, ok := u.NextCandidate(); ok {
		t.Fatal("empty argument position produced candidate")
	}
	must(u.GenCandidates(0, nullary, nil))
	candidate, ok := u.NextCandidate()
	if !ok || len(candidate.Children) != 0 {
		t.Fatal("nullary product")
	}
	parent, err := u.CreateNodeInstance(relation, 0)
	must(err)
	children := []*Node{a, b}
	_, err = u.CreateEdgeInstance(edge, parent, children)
	must(err)
	must(u.RegisterStratifiedLeaf(relation, parent))
	u.StratifiedInlining = true
	must(u.AddCandidate(edge, children))
	if _, ok = u.NextCandidate(); ok {
		t.Fatal("stratified duplicate queued")
	}
	must(u.AddCandidate(edge, []*Node{b, a}))
	candidate, ok = u.NextCandidate()
	if !ok || candidate.Children[0] != b {
		t.Fatal("distinct child tuple suppressed")
	}
	u.StratifiedInlining = false
	must(u.AddCandidate(edge, children))
	children[0] = b
	candidate, ok = u.NextCandidate()
	if !ok || candidate.Children[0] != a {
		t.Fatal("candidate did not copy children")
	}
}
