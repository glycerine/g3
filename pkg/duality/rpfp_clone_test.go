package duality

import "testing"

func TestCheckerClonesKeepEdgeLocalNodeCopiesAndIndependentAnnotations(t *testing.T) {
	ctx, _, m := dualityContextFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	term, err := ctx.BoolConst("edge-clone-R")
	must(err)
	closeExpr(t, term)
	before := m.Stats()
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	root, err := source.CreateNode(term)
	must(err)
	child, err := source.CreateNode(term)
	must(err)
	edge, err := source.CreateEdge(root, root.Annotation, []*Node{child, child})
	must(err)
	leaf, err := source.CreateLowerBoundEdge(child)
	must(err)
	target, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	defer target.Close()
	clones, err := NewRPFPCloneMap(target)
	must(err)
	must(clones.Clone(source))
	rootCopy, err := clones.GetEdgeClone(edge)
	must(err)
	leafCopy, err := clones.GetEdgeClone(leaf)
	must(err)
	if len(target.nodes) != 4 || len(target.edges) != 2 || rootCopy.Parent.Map != root || leafCopy.Parent.Map != child {
		t.Fatal("source per-edge cloning")
	}
	first, second := rootCopy.Children[0], rootCopy.Children[1]
	if first == second || first == leafCopy.Parent || second == leafCopy.Parent || first.Map != child || second.Map != child {
		t.Fatal("node copies incorrectly shared")
	}
	if first.Outgoing != nil || second.Outgoing != nil || leafCopy.Parent.Outgoing != leafCopy {
		t.Fatal("source edge-local wiring")
	}
	must(first.Annotation.SetEmpty())
	full, err := second.Annotation.IsFull()
	must(err)
	if !full {
		t.Fatal("child annotations share mutable state")
	}
	full, err = child.Annotation.IsFull()
	must(err)
	if !full {
		t.Fatal("clone changed original")
	}
	must(rootCopy.F.SetEmpty())
	full, err = edge.F.IsFull()
	must(err)
	if !full {
		t.Fatal("edge transformer shares mutable state")
	}
	missing, err := clones.GetEdgeClone(nil)
	must(err)
	if missing != nil {
		t.Fatal("missing clone is not null")
	}
	// Source Clone appends new local copies and replaces edge lookup entries.
	must(clones.Clone(source))
	updated, err := clones.GetEdgeClone(edge)
	must(err)
	if updated == rootCopy || len(target.nodes) != 8 || len(target.edges) != 4 {
		t.Fatal("repeated source clone behavior")
	}
	must(target.Close())
	must(source.Close())
	after := m.Stats()
	if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
		t.Fatal("checker cloning leaked", before, after)
	}
}
