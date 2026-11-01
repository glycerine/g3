package duality

import "testing"

func TestCollectiveCoverRequiresUnionAndInvalidatesOnUpdate(t *testing.T) {
	ctx, _, aux := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	x, err := ctx.BoolConst("union-cover-x")
	must(err)
	closeExpr(t, x)
	negative, err := x.Not()
	must(err)
	closeExpr(t, negative)
	sort, err := x.Sort()
	must(err)
	defer sort.Close()
	symbol, err := ctx.StringSymbol("union-cover-R")
	must(err)
	decl, err := ctx.Function(symbol, []*Sort{sort}, sort)
	must(err)
	closeDecl(t, decl)
	term, err := decl.Call(x)
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
	for n := 0; n < 3; n++ {
		node, err := working.CloneNode(source)
		must(err)
		_, err = working.CreateLowerBoundEdge(node)
		must(err)
		nodes = append(nodes, node)
	}
	for n, formula := range []*Expr{x, negative} {
		replacement, err := NewTransformer(ctx, nil, []*Expr{x}, formula)
		must(err)
		must(nodes[n].Annotation.Close())
		nodes[n].Annotation = replacement
	}
	covering, err := NewCovering(working, aux)
	must(err)
	for _, candidate := range nodes[:2] {
		valid, err := covering.CheckCover(nodes[2], candidate)
		must(err)
		if valid {
			t.Fatal("individual annotation unexpectedly covers true")
		}
	}
	must(covering.SetInstanceActive(nodes[1], false))
	blocked, err := covering.CoverByAll(nodes[2])
	must(err)
	if blocked {
		t.Fatal("inactive annotation contributed to union")
	}
	must(covering.SetInstanceActive(nodes[1], true))
	valid, err := covering.Close(nodes[2])
	must(err)
	if !valid || covering.coveredBy[nodes[2]] != nodes[2] {
		t.Fatal("collective cover not recorded")
	}
	valid, err = covering.Close(nodes[2])
	must(err)
	if !valid {
		t.Fatal("existing cover not recognized")
	}
	covering.Update(nodes[0])
	if covering.coveredBy[nodes[2]] != nil {
		t.Fatal("collective cover survived update")
	}
	must(nodes[0].Annotation.SetEmpty())
	valid, err = covering.CoverByAll(nodes[2])
	must(err)
	if valid {
		t.Fatal("incomplete union covered full relation")
	}
	valid, err = covering.Close(nodes[0])
	must(err)
	if valid {
		t.Fatal("node with no earlier candidates covered")
	}
}
