package duality

import (
	"testing"

	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func TestRPFPBackgroundTheorySolveLearnsOnlyUnderAxiom(t *testing.T) {
	ctx, m, native, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	manager, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
	must(err)
	defer func() { must(manager.Close()) }()

	background, err := ctx.BoolConst("solve-background-theory")
	must(err)
	closeExpr(t, background)
	backgroundDeclaration, err := background.Decl()
	must(err)
	closeDecl(t, backgroundDeclaration)
	notBackground, err := background.Not()
	must(err)
	closeExpr(t, notBackground)
	relation, err := ctx.BoolConst("solve-background-relation")
	must(err)
	closeExpr(t, relation)

	graph, err := NewRPFP(ctx)
	must(err)
	defer func() { must(graph.Close()) }()
	must(graph.BindSolver(native))
	must(graph.DeclareConstant(backgroundDeclaration))
	node, err := graph.CreateNode(relation)
	must(err)
	must(node.Bound.SetEmpty())
	transformer, err := NewTransformer(ctx, nil, nil, notBackground)
	must(err)
	defer func() { must(transformer.Close()) }()
	edge, err := graph.CreateEdge(node, transformer, nil)
	must(err)

	// Source background axioms are asserted before RPFP scopes and are also
	// classified as interpolation theory.
	must(native.Push())
	backgroundScope := true
	defer func() {
		if backgroundScope {
			must(native.Pop(1))
		}
	}()
	h, err := background.Raw()
	must(err)
	must(native.Assert(h))
	must(graph.Push())
	graphScope := true
	defer func() {
		if graphScope {
			must(graph.Pop(1))
		}
	}()
	must(graph.AssertNode(node))
	must(graph.AssertEdge(edge, 0, false, false))
	status, err := graph.solveWithTheory(node, manager, []*Expr{background}, nil)
	must(err)
	if status != lbool.False {
		t.Fatal("background-dependent graph was not safe", status)
	}
	learned, err := node.Annotation.Formula()
	must(err)
	defer func() { must(learned.Close()) }()
	must(graph.Pop(1))
	graphScope = false
	must(native.Pop(1))
	backgroundScope = false

	// Independently validate the learned relation under the background theory.
	must(native.Push())
	h, err = background.Raw()
	must(err)
	must(native.Assert(h))
	h, err = learned.Raw()
	must(err)
	must(native.Assert(h))
	status, err = native.CheckSat(nil)
	must(err)
	must(native.Pop(1))
	if status != lbool.False {
		t.Fatal("learned annotation is not false under the background", status)
	}

	// The identical graph is reachable without the axiom, proving that the
	// background rather than the edge alone caused the interpolation result.
	must(node.Annotation.SetFull())
	must(graph.Push())
	must(graph.AssertNode(node))
	must(graph.AssertEdge(edge, 0, false, false))
	status, err = graph.Solve(node, manager, nil)
	must(err)
	must(graph.Pop(1))
	if status != lbool.True || graph.dualModel == nil {
		t.Fatal("graph unexpectedly safe without background", status)
	}
}

func TestRPFPBackgroundTheorySingleNodeRefinement(t *testing.T) {
	ctx, m, native, globals := dualityNativeSolverServicesFixture(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	manager, err := interp.NewManagerWithParams(m, ctx.config.params, globals)
	must(err)
	defer func() { must(manager.Close()) }()

	background, err := ctx.BoolConst("single-background-theory")
	must(err)
	closeExpr(t, background)
	backgroundDeclaration, err := background.Decl()
	must(err)
	closeDecl(t, backgroundDeclaration)
	notBackground, err := background.Not()
	must(err)
	closeExpr(t, notBackground)
	parentTerm, err := ctx.BoolConst("single-background-parent")
	must(err)
	closeExpr(t, parentTerm)
	childTerm, err := ctx.BoolConst("single-background-child")
	must(err)
	closeExpr(t, childTerm)
	childDeclaration, err := childTerm.Decl()
	must(err)
	closeDecl(t, childDeclaration)

	graph, err := NewRPFP(ctx)
	must(err)
	defer func() { must(graph.Close()) }()
	must(graph.BindSolver(native))
	must(graph.DeclareConstant(backgroundDeclaration))
	parent, err := graph.CreateNode(parentTerm)
	must(err)
	must(parent.Bound.SetEmpty())
	child, err := graph.CreateNode(childTerm)
	must(err)
	parentTransformer, err := NewTransformer(ctx, []*FuncDecl{childDeclaration}, nil, childTerm)
	must(err)
	defer func() { must(parentTransformer.Close()) }()
	childTransformer, err := NewTransformer(ctx, nil, nil, notBackground)
	must(err)
	defer func() { must(childTransformer.Close()) }()
	parentEdge, err := graph.CreateEdge(parent, parentTransformer, []*Node{child})
	must(err)
	childEdge, err := graph.CreateEdge(child, childTransformer, nil)
	must(err)

	must(native.Push())
	backgroundScope := true
	defer func() {
		if backgroundScope {
			must(native.Pop(1))
		}
	}()
	h, err := background.Raw()
	must(err)
	must(native.Assert(h))
	must(graph.Push())
	graphScope := true
	defer func() {
		if graphScope {
			must(graph.Pop(1))
		}
	}()
	must(graph.AssertNode(parent))
	must(graph.AssertEdge(parentEdge, 0, false, false))
	must(graph.AssertEdge(childEdge, 0, false, false))
	status, err := graph.solveSingleNodeWithTheory(parent, child, manager, []*Expr{background}, nil)
	must(err)
	if status != lbool.False {
		t.Fatal("background-dependent subtree was not refined", status)
	}
	full, err := parent.Annotation.IsFull()
	must(err)
	if !full {
		t.Fatal("single-node refinement changed its surrounding parent")
	}
	learned, err := child.Annotation.Formula()
	must(err)
	defer func() { must(learned.Close()) }()
	must(graph.Pop(1))
	graphScope = false
	must(native.Pop(1))
	backgroundScope = false

	must(native.Push())
	h, err = background.Raw()
	must(err)
	must(native.Assert(h))
	h, err = learned.Raw()
	must(err)
	must(native.Assert(h))
	status, err = native.CheckSat(nil)
	must(err)
	must(native.Pop(1))
	if status != lbool.False {
		t.Fatal("refined child is not false under the background", status)
	}

	must(child.Annotation.SetFull())
	must(graph.Push())
	must(graph.AssertNode(parent))
	must(graph.AssertEdge(parentEdge, 0, false, false))
	must(graph.AssertEdge(childEdge, 0, false, false))
	status, err = graph.SolveSingleNode(parent, child, manager, nil)
	must(err)
	must(graph.Pop(1))
	if status != lbool.True || graph.dualModel == nil {
		t.Fatal("subtree unexpectedly safe without background", status)
	}
}
