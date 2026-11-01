package duality

import "testing"

func TestCoveringDominanceUsesMappedEdgesAndRecordedChildren(t *testing.T) {
	ctx, _, aux := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	term, err := ctx.BoolConst("dominance-R")
	must(err)
	closeExpr(t, term)
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	parentSource, err := source.CreateNode(term)
	must(err)
	childSource, err := source.CreateNode(term)
	must(err)
	parentMap, err := source.CreateEdge(parentSource, parentSource.Annotation, []*Node{childSource})
	must(err)
	childMap, err := source.CreateLowerBoundEdge(childSource)
	must(err)
	graph, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	defer graph.Close()
	makeNode := func(original *Node) *Node { node, err := graph.CloneNode(original); must(err); return node }
	oldChild, newChild := makeNode(childSource), makeNode(childSource)
	oldLeaf, err := graph.CreateLowerBoundEdge(oldChild)
	must(err)
	newLeaf, err := graph.CreateLowerBoundEdge(newChild)
	must(err)
	newLeaf.Map = childMap
	oldParent, newParent := makeNode(parentSource), makeNode(parentSource)
	oldEdge, err := graph.CreateEdge(oldParent, oldParent.Annotation, []*Node{oldChild})
	must(err)
	oldEdge.Map = parentMap
	newEdge, err := graph.CreateEdge(newParent, newParent.Annotation, []*Node{newChild})
	must(err)
	newEdge.Map = parentMap
	covering, err := NewCovering(graph, aux)
	must(err)
	self, err := covering.Dominates(newParent, newParent)
	must(err)
	if self {
		t.Fatal("self dominance")
	}
	// An unmapped child expansion is an approximation and can be dominated.
	possible, err := covering.Dominates(newParent, oldParent)
	must(err)
	if !possible {
		t.Fatal("approximate child did not permit dominance")
	}
	oldLeaf.Map = childMap
	possible, err = covering.Dominates(newParent, oldParent)
	must(err)
	if possible {
		t.Fatal("unrecorded mapped child dominance accepted")
	}
	oldLeaf.Map = nil
	must(covering.Add(newChild))
	if !covering.dominance[newChild][oldChild] {
		t.Fatal("child dominance not recorded")
	}
	// Once both children have mapped edges, the recorded relation is required.
	oldLeaf.Map = childMap
	possible, err = covering.Dominates(newParent, oldParent)
	must(err)
	if !possible {
		t.Fatal("recorded child dominance ignored")
	}
	must(covering.Add(newParent))
	if !covering.dominance[newParent][oldParent] {
		t.Fatal("parent dominance not recorded")
	}
	candidate, err := covering.Candidate(oldParent)
	must(err)
	if candidate {
		t.Fatal("dominated instance remains candidate")
	}
	contains, err := covering.Contains(oldParent)
	must(err)
	if !contains {
		t.Fatal("structural dominance changed covering membership")
	}
	// A different source expansion is incomparable.
	alternateMap, err := source.CreateEdge(parentSource, parentSource.Annotation, []*Node{childSource})
	must(err)
	oldEdge.Map = alternateMap
	possible, err = covering.Dominates(newParent, oldParent)
	must(err)
	if possible {
		t.Fatal("different source edges compared as equal")
	}
}
