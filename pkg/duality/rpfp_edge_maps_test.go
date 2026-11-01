package duality

import "testing"

func TestRPFPEdgeMapsRefreshAndDuplicateParameters(t *testing.T) {
	ctx, _, m := dualityContextFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	x, err := ctx.BoolConst("map-x")
	must(err)
	closeExpr(t, x)
	y, err := ctx.BoolConst("map-y")
	must(err)
	closeExpr(t, y)
	sort, err := x.Sort()
	must(err)
	defer sort.Close()
	name, err := ctx.StringSymbol("map-R")
	must(err)
	decl, err := ctx.Function(name, []*Sort{sort, sort}, sort)
	must(err)
	closeDecl(t, decl)
	term, err := decl.Call(x, y)
	must(err)
	closeExpr(t, term)
	formula, err := NewTransformer(ctx, []*FuncDecl{decl, decl}, []*Expr{x, x}, x)
	must(err)
	defer func() { must(formula.Close()) }()
	before := m.Stats()
	g, err := NewRPFP(ctx)
	must(err)
	defer func() { must(g.Close()) }()
	node, err := g.CreateNode(term)
	must(err)
	edge, err := g.CreateEdge(node, formula, []*Node{node, node})
	must(err)
	must(g.SetEdgeMaps(edge))
	dh, err := decl.Raw()
	must(err)
	if len(edge.relMap) != 1 || edge.relMap[dh] != 1 {
		t.Fatal("source last relational parameter must win", edge.relMap)
	}
	xh, err := x.Raw()
	must(err)
	mapped, err := m.RefGet(edge.varMap[xh])
	must(err)
	expected, err := SuffixVariable(y, node.Number)
	must(err)
	want, err := expected.Raw()
	must(err)
	if len(edge.varMap) != 1 || mapped != want {
		t.Fatal("source last individual parameter must win")
	}
	must(expected.Close())
	snapshot := m.Stats()
	for n := 0; n < 3; n++ {
		must(g.SetEdgeMaps(edge))
		after := m.Stats()
		if after.LiveNodes != snapshot.LiveNodes || after.LiveReferences != snapshot.LiveReferences {
			t.Fatal("refresh leaked", snapshot, after)
		}
	}
	// Refresh uses the current annotation parameters rather than stale copies.
	replacement, err := NewTransformer(ctx, nil, []*Expr{y, x}, x)
	must(err)
	must(node.Annotation.Close())
	node.Annotation = replacement
	must(g.SetEdgeMaps(edge))
	expected, err = SuffixVariable(x, node.Number)
	must(err)
	want, err = expected.Raw()
	must(err)
	mapped, err = m.RefGet(edge.varMap[xh])
	must(err)
	if mapped != want {
		t.Fatal("refresh ignored changed parent annotation")
	}
	must(expected.Close())
	must(g.Close())
	after := m.Stats()
	if after.LiveNodes != before.LiveNodes || after.LiveReferences != before.LiveReferences {
		t.Fatal("edge maps leaked", before, after)
	}
}
