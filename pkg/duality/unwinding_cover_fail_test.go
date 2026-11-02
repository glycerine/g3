package duality

import (
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/model"
	"testing"
)

func TestDualityCoverFailureBuildsLeafRedirectsAndExpandsNative(t *testing.T) {
	ctx, m, native, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	manager, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
	must(err)
	defer manager.Close()
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	relationTerm, err := ctx.BoolConst("cover-fail-R")
	must(err)
	closeExpr(t, relationTerm)
	parentTerm, err := ctx.BoolConst("cover-fail-parent")
	must(err)
	closeExpr(t, parentTerm)
	leafTerm, err := ctx.BoolConst("cover-fail-leaf")
	must(err)
	closeExpr(t, leafTerm)
	relationDecl, err := relationTerm.Decl()
	must(err)
	defer relationDecl.Close()
	leafDecl, err := leafTerm.Decl()
	must(err)
	defer leafDecl.Close()
	relation, err := source.CreateNode(relationTerm)
	must(err)
	parentSource, err := source.CreateNode(parentTerm)
	must(err)
	leafSource, err := source.CreateNode(leafTerm)
	must(err)
	relationRule, err := NewTransformer(ctx, []*FuncDecl{leafDecl}, nil, leafTerm)
	must(err)
	defer relationRule.Close()
	relationEdge, err := source.CreateEdge(relation, relationRule, []*Node{leafSource})
	must(err)
	parentRule, err := NewTransformer(ctx, []*FuncDecl{relationDecl}, nil, relationTerm)
	must(err)
	defer parentRule.Close()
	parentEdge, err := source.CreateEdge(parentSource, parentRule, []*Node{relation})
	must(err)
	graph, err := NewRPFPWithVocabulary(source.vocabulary)
	must(err)
	defer graph.Close()
	covering, err := NewCovering(graph, native)
	must(err)
	u, err := NewUnwinding(source, graph, covering)
	must(err)
	must(u.CreateEdgesByChildMap())
	leaf, err := u.CreateNodeInstance(leafSource, 0)
	must(err)
	_, err = graph.CreateLowerBoundEdge(leaf)
	must(err)
	other, err := u.CreateNodeInstance(relation, 0)
	must(err)
	_, err = u.CreateEdgeInstance(relationEdge, other, []*Node{leaf})
	must(err)
	must(u.ActivateInstance(other))
	node, err := u.CreateNodeInstance(relation, 0)
	must(err)
	_, err = u.CreateEdgeInstance(relationEdge, node, []*Node{leaf})
	must(err)
	parent, err := u.CreateNodeInstance(parentSource, 0)
	must(err)
	_, err = u.CreateEdgeInstance(parentEdge, parent, []*Node{other})
	must(err)
	// A useful old fact exists, so the source similar-node search is enabled.
	must(covering.Update(node))
	u.backEdges[node] = map[*Node]int{relation: 4}
	cexGraph, err := NewRPFP(ctx)
	must(err)
	root, err := cexGraph.CreateNode(relationTerm)
	must(err)
	must(root.Underapprox.SetFull())
	cex := NewCounterexample()
	defer cex.Close()
	must(cex.Set(cexGraph, root))
	must(native.Push())
	caller, err := ctx.BoolConst("cover-fail-caller")
	must(err)
	closeExpr(t, caller)
	raw, err := caller.Raw()
	must(err)
	must(native.Assert(raw))
	_, err = native.CheckSat(nil)
	must(err)
	baseline, err := native.NumAssertions()
	must(err)
	expanded, err := u.ExpandNodeFromCoverFail(node, cex, native, derivationEvaluatorFixture(t, ctx, globals), manager)
	must(err)
	if !expanded {
		t.Fatal("cover failure did not expand")
	}
	under := covering.coveredBy[node]
	if under == nil || covering.underapprox[under] != node || under.Map != relation || under.Outgoing.Map != nil || len(under.Outgoing.Children) != 0 || u.backEdges[under][relation] != 4 {
		t.Fatal("under-leaf association/cover/edge/count mismatch")
	}
	full, err := under.Annotation.IsFull()
	must(err)
	if !full {
		t.Fatal("covering under-leaf annotation not full")
	}
	active := false
	for _, instance := range covering.instances[relation] {
		if instance == under {
			active = true
		}
	}
	if !active {
		t.Fatal("under-leaf was not activated")
	}
	for _, pending := range u.PendingNodes() {
		if pending == under {
			t.Fatal("activated leaf left pending")
		}
	}
	redirected, ok := u.NextCandidate()
	if !ok || redirected.Edge != parentEdge || len(redirected.Children) != 1 || redirected.Children[0] != under {
		t.Fatal("similar-node redirection missing")
	}
	generated, ok := u.NextCandidate()
	if !ok || generated.Edge != parentEdge || len(generated.Children) != 1 || (generated.Children[0] != under && generated.Children[0] != other) {
		t.Fatal("native induction candidate missing")
	}
	if _, ok = u.NextCandidate(); ok {
		t.Fatal("unexpected extra candidates")
	}
	if cex.Tree() != cexGraph || root.closed || cexGraph.closed {
		t.Fatal("cover-failure expansion consumed borrowed counterexample")
	}
	depth, err := native.ScopeLevel()
	must(err)
	assertions, err := native.NumAssertions()
	must(err)
	if depth != 1 || assertions != baseline {
		t.Fatal("cover failure disturbed caller scopes/assertions")
	}
	must(native.Pop(1))
}

func TestDualityCoverFailureShortcutsNeedNoCounterexampleOrSolver(t *testing.T) {
	ctx, _, native := dualityNativeSolverFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	source, err := NewRPFP(ctx)
	must(err)
	defer source.Close()
	term, err := ctx.BoolConst("cover-fail-shortcut")
	must(err)
	closeExpr(t, term)
	relation, err := source.CreateNode(term)
	must(err)
	graph, err := NewRPFP(ctx)
	must(err)
	defer graph.Close()
	covering, err := NewCovering(graph, native)
	must(err)
	u, err := NewUnwinding(source, graph, covering)
	must(err)
	node, err := u.CreateNodeInstance(relation, 0)
	must(err)
	// No outgoing edge and then a nullary edge both stop before services.
	call := func() {
		t.Helper()
		expanded, err := u.ExpandNodeFromCoverFail(node, nil, nil, model.ModelEvaluatorConfig{}, nil)
		must(err)
		if expanded {
			t.Fatal("shortcut expanded")
		}
	}
	call()
	_, err = graph.CreateLowerBoundEdge(node)
	must(err)
	call()
	// A nonleaf still stops if updates/similar instances are absent.
	_, err = graph.CreateEdge(node, node.Annotation, []*Node{node})
	must(err)
	call()
	covering.someUpdates = true
	call()
	if len(graph.nodes) != 1 || len(covering.underapprox) != 0 || len(u.candidates) != 0 {
		t.Fatal("shortcut mutated search")
	}
}
