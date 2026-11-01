package duality

import "testing"

func TestCoveringNativeContainmentAndUpdateInvalidation(t *testing.T) {
	ctx, _, aux := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	term, err := ctx.BoolConst("cover-R")
	must(err)
	closeExpr(t, term)
	original, err := NewRPFP(ctx)
	must(err)
	defer original.Close()
	source, err := original.CreateNode(term)
	must(err)
	working, err := NewRPFPWithVocabulary(original.vocabulary)
	must(err)
	defer working.Close()
	var nodes []*Node
	for n := 0; n < 5; n++ {
		node, err := working.CloneNode(source)
		must(err)
		_, err = working.CreateLowerBoundEdge(node)
		must(err)
		nodes = append(nodes, node)
	}
	covering, err := NewCovering(working, aux)
	must(err)
	added, err := covering.CoverByNode(nodes[2], nodes[1])
	must(err)
	if !added {
		t.Fatal("valid covering rejected")
	}
	covered, err := covering.IsCovered(nodes[2])
	must(err)
	if !covered {
		t.Fatal("covering not recorded")
	}
	added, err = covering.CoverByNode(nodes[1], nodes[2])
	must(err)
	if added {
		t.Fatal("reverse source order accepted")
	}
	// A covering candidate whose descendant is covered is unavailable.
	must(working.DeleteEdge(nodes[3].Outgoing))
	_, err = working.CreateEdge(nodes[3], nodes[3].Annotation, []*Node{nodes[2]})
	must(err)
	added, err = covering.CoverByNode(nodes[4], nodes[3])
	must(err)
	if added {
		t.Fatal("transitively covered candidate accepted")
	}
	covering.Update(nodes[1])
	if !covering.someUpdates {
		t.Fatal("update flag not set")
	}
	covered, err = covering.IsCovered(nodes[2])
	must(err)
	if covered {
		t.Fatal("stale covering survived annotation update")
	}
	// A stronger covering annotation must really contain the covered fact.
	must(nodes[1].Annotation.SetEmpty())
	added, err = covering.CoverByNode(nodes[2], nodes[1])
	must(err)
	if added {
		t.Fatal("non-containing annotation accepted")
	}
	must(nodes[1].Annotation.SetFull())
	added, err = covering.CoverByNode(nodes[2], nodes[1])
	must(err)
	if !added {
		t.Fatal("covering not restored")
	}
	// Underapproximation nodes may cover their associated original out of order,
	// but an underapproximation node may never itself be covered.
	must(covering.MapUnderapproximation(nodes[4], nodes[0]))
	if !covering.CoverOrder(nodes[4], nodes[0]) || covering.CoverOrder(nodes[0], nodes[4]) {
		t.Fatal("underapproximation covering order")
	}
	// Ascendant invalidation visits parents and stops on graph cycles.
	must(working.DeleteEdge(nodes[0].Outgoing))
	_, err = working.CreateEdge(nodes[0], nodes[0].Annotation, []*Node{nodes[3]})
	must(err)
	must(working.DeleteEdge(nodes[3].Outgoing))
	_, err = working.CreateEdge(nodes[3], nodes[3].Annotation, []*Node{nodes[0]})
	must(err)
	covering.RemoveAscendantCoverings(nodes[3])
	covered, err = covering.IsCovered(nodes[2])
	must(err)
	if covered {
		t.Fatal("ancestor invalidation missed later instance")
	}
	covered, err = covering.IsCovered(nodes[0])
	must(err)
	if covered {
		t.Fatal("uncovered cycle reported covered")
	}
}
