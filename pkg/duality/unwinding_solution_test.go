package duality

import (
	"github.com/glycerine/g3/pkg/interp"
	"testing"
)

func TestUnwindingSolutionUnionsActiveUncoveredAnnotations(t *testing.T) {
	ctx, m, aux, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	simplifier, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
	must(err)
	defer simplifier.Close()
	term, err := ctx.BoolConst("solution-R")
	must(err)
	closeExpr(t, term)
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	relation, err := source.CreateNode(term)
	must(err)
	edge, err := source.CreateLowerBoundEdge(relation)
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
	_, err = u.CreateEdgeInstance(edge, a, nil)
	must(err)
	b, err := u.CreateNodeInstance(relation, 0)
	must(err)
	_, err = u.CreateEdgeInstance(edge, b, nil)
	must(err)
	destination, err := relation.Annotation.Clone()
	must(err)
	defer destination.Close()
	must(u.GenNodeSolutionFromIndSet(relation, destination, false, simplifier))
	empty, err := destination.IsEmpty()
	must(err)
	if !empty {
		t.Fatal("inactive instances contributed")
	}
	must(u.ActivateInstance(a))
	full, err := u.NodeSolutionFromIndSetFull(relation)
	must(err)
	if !full {
		t.Fatal("active full annotation missed")
	}
	must(u.GenNodeSolutionFromIndSet(relation, destination, false, simplifier))
	full, err = destination.IsFull()
	must(err)
	if !full {
		t.Fatal("full solution union")
	}
	// Marking a true annotation yields its source instance marker after rewriting.
	must(u.GenNodeSolutionFromIndSet(relation, destination, true, simplifier))
	marker, err := NodeMarker(a, nil)
	must(err)
	defer marker.Close()
	formula, err := destination.Formula()
	must(err)
	h, err := formula.Raw()
	must(err)
	want, err := marker.Raw()
	must(err)
	if h != want {
		t.Fatal("marked union did not identify instance")
	}
	must(formula.Close())
	pair, err := NodeMarker(a, b)
	must(err)
	defer pair.Close()
	decl, err := pair.Decl()
	must(err)
	name, err := decl.Name()
	must(err)
	text, err := name.String()
	must(err)
	must(decl.Close())
	if text != "@m_1_2" {
		t.Fatal("pair marker name", text)
	}
	must(u.ActivateInstance(b))
	added, err := covering.CoverByNode(b, a)
	must(err)
	if !added {
		t.Fatal("initial covering")
	}
	must(a.Annotation.SetEmpty()) // B remains covered until the outer Update invalidates it.
	must(u.GenNodeSolutionFromIndSet(relation, destination, false, simplifier))
	empty, err = destination.IsEmpty()
	must(err)
	if !empty {
		t.Fatal("covered instance contributed to solution")
	}
	full, err = u.NodeSolutionFromIndSetFull(relation)
	must(err)
	if full {
		t.Fatal("covered full annotation counted")
	}
}
