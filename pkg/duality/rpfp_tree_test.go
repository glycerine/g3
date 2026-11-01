package duality

import "testing"

func TestRPFPGoalTreeAndInterpolationDecode(t *testing.T) {
	ctx, _, m := dualityContextFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	x, err := ctx.BoolConst("tree-x")
	must(err)
	closeExpr(t, x)
	sort, err := x.Sort()
	must(err)
	defer sort.Close()
	symbol, err := ctx.StringSymbol("tree-R")
	must(err)
	decl, err := ctx.Function(symbol, []*Sort{sort}, sort)
	must(err)
	closeDecl(t, decl)
	term, err := decl.Call(x)
	must(err)
	closeExpr(t, term)
	before := m.Stats()
	g, err := NewRPFP(ctx)
	must(err)
	defer func() { must(g.Close()) }()
	parent, err := g.CreateNode(term)
	must(err)
	child, err := g.CreateNode(term)
	must(err)
	_, err = g.CreateEdge(parent, parent.Annotation, []*Node{child})
	must(err)
	goal, err := g.GetGoalTree(parent)
	must(err)
	if goal.Number(7) != 10 || goal.GetNumber() != 9 || goal.children[0].GetNumber() != 8 || goal.children[0].children[0].GetNumber() != 7 {
		t.Fatal("postorder numbering")
	}
	rootTerm, err := goal.Term()
	must(err)
	h, err := rootTerm.Raw()
	must(err)
	if !m.IsFalse(h) {
		t.Fatal("goal wrapper is not false")
	}
	must(rootTerm.Close())
	for n, node := range []*Node{parent, child} {
		tree := goal.children[0]
		if n == 1 {
			tree = tree.children[0]
		}
		expectedVar, err := SuffixVariable(x, node.Number)
		must(err)
		expected, err := decl.Call(expectedVar)
		must(err)
		got, err := tree.Term()
		must(err)
		a, err := got.Raw()
		must(err)
		b, err := expected.Raw()
		must(err)
		if a != b {
			t.Fatal("goal tree used wrong node vocabulary", n)
		}
		must(got.Close())
		must(expected.Close())
		must(expectedVar.Close())
	}
	must(goal.Close())
	// Construct a tree of learned indexed facts and decode it into the graph.
	indexedParent, err := SuffixVariable(x, parent.Number)
	must(err)
	indexedChild, err := SuffixVariable(x, child.Number)
	must(err)
	negative, err := indexedChild.Not()
	must(err)
	leaf, err := NewTermTree(negative, nil)
	must(err)
	learned, err := NewTermTree(indexedParent, []*TermTree{leaf})
	must(err)
	must(learned.AddTerm(indexedChild))
	must(indexedParent.Close())
	must(indexedChild.Close())
	must(negative.Close())
	must(g.DecodeTree(parent, learned))
	annotation, err := parent.Annotation.Formula()
	must(err)
	got, err := annotation.Raw()
	must(err)
	want, err := x.Raw()
	must(err)
	if got != want {
		t.Fatal("parent interpolant not decoded")
	}
	must(annotation.Close())
	expected, err := x.Not()
	must(err)
	annotation, err = child.Annotation.Formula()
	must(err)
	got, err = annotation.Raw()
	must(err)
	want, err = expected.Raw()
	must(err)
	if got != want {
		t.Fatal("child interpolant not decoded")
	}
	must(annotation.Close())
	must(expected.Close())
	// A leaf interpolation result updates only that node, matching the source.
	truth, err := ctx.BoolVal(true)
	must(err)
	must(learned.SetTerm(truth))
	flat, err := NewTermTree(truth, nil)
	must(err)
	must(g.DecodeTree(parent, flat))
	must(flat.Close())
	full, err := parent.Annotation.IsFull()
	must(err)
	if !full {
		t.Fatal("leaf result did not update parent")
	}
	still, err := child.Annotation.Formula()
	must(err)
	sh, err := still.Raw()
	must(err)
	if m.IsTrue(sh) {
		t.Fatal("leaf result changed child")
	}
	must(still.Close())
	must(truth.Close())
	must(learned.Close())
	must(learned.Close())
	must(g.Close())
	after := m.Stats()
	if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
		t.Fatal("tree ownership leaked", before, after)
	}
}
