package duality

import (
	"github.com/glycerine/g3/pkg/ast"
	"testing"
)

func TestDualityUnderapproxLeafCapturesFactAndCopiesBackEdges(t *testing.T) {
	ctx, m, aux := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	term, err := ctx.BoolConst("under-node-R")
	must(err)
	closeExpr(t, term)
	relation, err := source.CreateNode(term)
	must(err)
	graph, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	defer graph.Close()
	covering, err := NewCovering(graph, aux)
	must(err)
	outer, err := NewUnwinding(source, graph, covering)
	must(err)
	node, err := outer.CreateNodeInstance(relation, 0)
	must(err)
	outer.backEdges[node] = map[*Node]int{relation: 3}
	cexGraph, err := NewRPFP(ctx)
	must(err)
	root, err := cexGraph.CreateNode(term)
	must(err)
	cex := NewCounterexample()
	defer cex.Close()
	must(cex.Set(cexGraph, root))
	p, err := ctx.BoolConst("under-node-p")
	must(err)
	closeExpr(t, p)
	q, err := ctx.BoolConst("under-node-q")
	must(err)
	closeExpr(t, q)
	pair, err := p.And(q)
	must(err)
	closeExpr(t, pair)
	raw, err := pair.Raw()
	must(err)
	must(m.SetRef(root.Underapprox.formula, raw))
	under, err := outer.CreateUnderapproxNode(node, cex)
	must(err)
	full, err := under.Annotation.IsFull()
	must(err)
	if !full || under.Map != relation || under.Outgoing == nil || under.Outgoing.Map != nil || len(under.Outgoing.Children) != 0 {
		t.Fatal("under-node initialization")
	}
	marker, err := ctx.BoolConst("@thing")
	must(err)
	closeExpr(t, marker)
	ph, err := p.Raw()
	must(err)
	mh, err := marker.Raw()
	must(err)

	truth, err := m.MkTrue()
	must(err)
	expected, err := m.MkAndArgs([]ast.ASTHandle{truth, raw, mh})
	must(err)
	got, err := m.RefGet(under.Outgoing.F.formula)
	must(err)
	if got != expected {
		t.Fatal("lower edge lost source conjunction/marker", got, expected)
	}
	pairFact, err := NewTransformer(ctx, nil, nil, pair)
	must(err)
	defer pairFact.Close()
	implies, err := under.Outgoing.F.SubsetEq(pairFact, aux)
	must(err)
	reverse, err := pairFact.SubsetEq(under.Outgoing.F, aux)
	must(err)
	if !implies || reverse {
		t.Fatal("lower rule lost fact or local marker constraint")
	}
	if outer.backEdges[under][relation] != 3 {
		t.Fatal("missing recursion counts")
	}
	outer.backEdges[node][relation] = 8
	if outer.backEdges[under][relation] != 3 {
		t.Fatal("recursion counts alias original")
	}
	if len(covering.instances[relation]) != 0 || len(covering.allOfNode[relation]) != 2 {
		t.Fatal("under-node should remain pending until caller activates")
	}
	if len(covering.underapprox) != 0 {
		t.Fatal("creation registered caller-owned association")
	}
	must(cex.Close())
	if _, err = m.RefGet(under.Outgoing.F.formula); err != nil {
		t.Fatal("edge borrowed released counterexample fact", err)
	}
	// AddThing itself leaves atoms unchanged.
	atom, err := NewTransformer(ctx, nil, nil, p)
	must(err)
	defer atom.Close()
	must(outer.addUnderapproxThing(atom))
	got, err = m.RefGet(atom.formula)
	must(err)
	if got != ph {
		t.Fatal("marker added to nonconjunction")
	}
}

func TestDualityUnderapproxCandidatesRedirectAllPositionsDepthFirst(t *testing.T) {
	ctx, _, aux := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	term, err := ctx.BoolConst("under-candidate-R")
	must(err)
	closeExpr(t, term)
	relation, err := source.CreateNode(term)
	must(err)
	firstSource, err := source.CreateEdge(relation, relation.Annotation, []*Node{relation, relation})
	must(err)
	secondSource, err := source.CreateEdge(relation, relation.Annotation, []*Node{relation})
	must(err)
	graph, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	defer graph.Close()
	covering, err := NewCovering(graph, aux)
	must(err)
	outer, err := NewUnwinding(source, graph, covering)
	must(err)
	other, err := outer.CreateNodeInstance(relation, 0)
	must(err)
	replacement, err := outer.CreateNodeInstance(relation, 0)
	must(err)
	parent, err := outer.CreateNodeInstance(relation, 0)
	must(err)
	first, err := outer.CreateEdgeInstance(firstSource, parent, []*Node{other, other})
	must(err)
	second, err := outer.CreateEdgeInstance(secondSource, parent, []*Node{other})
	must(err)
	must(outer.AddCandidate(firstSource, []*Node{other, replacement}))
	// Source visits repeated incoming entries (one per child position) and uses
	// push_front, so second comes before both copies of first.
	outer.StratifiedInlining = true
	must(outer.RegisterStratifiedLeaf(relation, parent))
	must(outer.ExpandNodeFromOther(replacement, other))
	for i, want := range []*Edge{secondSource, firstSource, firstSource} {
		candidate, ok := outer.NextCandidate()
		if !ok || candidate.Edge != want {
			t.Fatal("depth-first order", i)
		}
		for _, child := range candidate.Children {
			if child != replacement {
				t.Fatal("repeated position not redirected")
			}
		}
		candidate.Children[0] = other
	}
	queued, ok := outer.NextCandidate()
	if !ok || queued.Children[0] != other || queued.Children[1] != replacement {
		t.Fatal("existing queue entry changed")
	}
	if _, ok = outer.NextCandidate(); ok {
		t.Fatal("extra candidate")
	}
	if first.Children[0] != other || first.Children[1] != other || second.Children[0] != other {
		t.Fatal("candidate redirection modified graph edges")
	}
}
