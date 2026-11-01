package duality

import "testing"

func TestRPFPConstraintTreeCopiesAssertedStateAndSkipsSubtrees(t *testing.T) {
	ctx, _, m := dualityContextFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	term, err := ctx.BoolConst("constraint-R")
	must(err)
	closeExpr(t, term)
	extra, err := ctx.BoolConst("constraint-extra")
	must(err)
	closeExpr(t, extra)
	before := m.Stats()
	g, err := NewRPFP(ctx)
	must(err)
	defer func() { must(g.Close()) }()
	root, err := g.CreateNode(term)
	must(err)
	leaf, err := g.CreateNode(term)
	must(err)
	edge, err := g.CreateEdge(root, root.Annotation, []*Node{leaf})
	must(err)
	open, err := g.ToTermTree(root, nil)
	must(err)
	h, err := open.term.Raw()
	must(err)
	if !m.IsTrue(h) || len(open.children) != 1 {
		t.Fatal("unasserted tree shape")
	}
	must(open.Close())
	dual, err := g.ReducedDualEdge(edge)
	must(err)
	dh, err := dual.Raw()
	must(err)
	edge.dual, err = m.NewRef(dh)
	must(err)
	must(dual.Close())
	eh, err := extra.Raw()
	must(err)
	ref, err := m.NewRef(eh)
	must(err)
	edge.constraints = append(edge.constraints, ref)
	upper, err := g.GetUpperBound(root)
	must(err)
	uh, err := upper.Raw()
	must(err)
	root.dual, err = m.NewRef(uh)
	must(err)
	must(upper.Close())
	tree, err := g.ToTermTree(root, nil)
	must(err)
	got, err := tree.term.Raw()
	must(err)
	if got != dh || len(tree.terms) != 1 || len(tree.children) != 1 {
		t.Fatal("asserted state not copied")
	}
	got, err = tree.terms[0].Raw()
	must(err)
	if got != eh {
		t.Fatal("extra constraint missing")
	}
	skipped, err := g.ToTermTree(root, root)
	must(err)
	got, err = skipped.term.Raw()
	must(err)
	if !m.IsTrue(got) || len(skipped.children) != 0 || len(skipped.terms) != 0 {
		t.Fatal("skipped subtree not true leaf")
	}
	must(skipped.Close())
	wrapper, err := g.AddUpperBound(root, tree)
	must(err)
	got, err = wrapper.term.Raw()
	must(err)
	if got != uh || wrapper.children[0] != tree {
		t.Fatal("upper-bound wrapper")
	}
	// Snapshot trees keep independent references after graph destruction.
	must(g.Close())
	got, err = tree.term.Raw()
	must(err)
	if got != dh {
		t.Fatal("graph destruction invalidated tree")
	}
	must(wrapper.Close())
	must(wrapper.Close())
	after := m.Stats()
	if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
		t.Fatal("constraint tree leaked", before, after)
	}
}
