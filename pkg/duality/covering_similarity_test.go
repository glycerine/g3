package duality

import "testing"

func TestCoveringSimilarityUsesActiveReverseOrderAndSemanticEmptiness(t *testing.T) {
	ctx, _, aux := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	term, err := ctx.BoolConst("similar-R")
	must(err)
	closeExpr(t, term)
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	relation, err := source.CreateNode(term)
	must(err)
	working, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	defer working.Close()
	var nodes []*Node
	for n := 0; n < 3; n++ {
		node, err := working.CloneNode(relation)
		must(err)
		_, err = working.CreateLowerBoundEdge(node)
		must(err)
		nodes = append(nodes, node)
	}
	covering, err := NewCovering(working, aux)
	must(err)
	similar, err := covering.GetSimilarNode(nodes[2])
	must(err)
	if similar != nil {
		t.Fatal("similarity enabled before updates")
	}
	covering.Update(nodes[0])
	similar, err = covering.GetSimilarNode(nodes[2])
	must(err)
	if similar != nodes[1] {
		t.Fatal("wrong reverse-order instance")
	}
	must(covering.SetInstanceActive(nodes[1], false))
	similar, err = covering.GetSimilarNode(nodes[2])
	must(err)
	if similar != nodes[0] {
		t.Fatal("inactive instance selected")
	}
	must(covering.SetInstanceActive(nodes[1], true))
	possible, err := covering.CouldCover(nodes[2], nodes[1])
	must(err)
	if !possible {
		t.Fatal("nonempty earlier candidate rejected")
	}
	// Use a syntactically non-false contradiction to require semantic checking.
	p, err := ctx.BoolConst("similar-p")
	must(err)
	closeExpr(t, p)
	neg, err := p.Not()
	must(err)
	closeExpr(t, neg)
	contradiction, err := p.And(neg)
	must(err)
	closeExpr(t, contradiction)
	empty, err := NewTransformer(ctx, nil, nil, contradiction)
	must(err)
	must(nodes[1].Annotation.Close())
	nodes[1].Annotation = empty
	possible, err = covering.CouldCover(nodes[2], nodes[1])
	must(err)
	if possible {
		t.Fatal("semantic empty candidate accepted")
	}
	similar, err = covering.GetSimilarNode(nodes[2])
	must(err)
	if similar != nodes[1] {
		t.Fatal("active similarity added a non-source emptiness filter")
	}
}
func TestCoveringConjectureExcludesSimpleSourceLoop(t *testing.T) {
	ctx, _, aux := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	term, err := ctx.BoolConst("simple-loop-R")
	must(err)
	closeExpr(t, term)
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	relation, err := source.CreateNode(term)
	must(err)
	_, err = source.CreateEdge(relation, relation.Annotation, []*Node{relation})
	must(err)
	_, err = source.CreateLowerBoundEdge(relation)
	must(err)
	working, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	defer working.Close()
	earlier, err := working.CloneNode(relation)
	must(err)
	_, err = working.CreateLowerBoundEdge(earlier)
	must(err)
	later, err := working.CloneNode(relation)
	must(err)
	_, err = working.CreateLowerBoundEdge(later)
	must(err)
	covering, err := NewCovering(working, aux)
	must(err)
	possible, err := covering.CouldCover(later, earlier)
	must(err)
	if possible {
		t.Fatal("simple loop accepted as conjecture candidate")
	}
}
